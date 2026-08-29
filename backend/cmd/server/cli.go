package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"pornhub.singles/server/internal/config"
	"pornhub.singles/server/internal/logging"
	"pornhub.singles/server/internal/store"
)

// Account management lives on the same binary as the server because the
// container image has nothing else in it:
//
//	docker compose exec app phs-server user list
//	docker compose exec app phs-server user grant-admin bob
//
// Role changes are deliberately unreachable over HTTP. Ownership in particular
// can only be moved from here, which is what makes the owner account immune to
// anything an administrator can do through the panel.

const cliUsage = `Usage: phs-server user <command> [arguments]

  list                     show every account with its role and badges
  create <username>        create an account (role: admin unless --role=member)
  delete <username>        remove an account (never the owner)
  passwd <username>        set a new password and sign the account out everywhere
  grant-admin <username>   give administrative privileges (adds the Administrator badge)
  revoke-admin <username>  take them away again (demotes to member)
  set-owner <username>     transfer ownership, including the page, to this account
  verify <username>        grant the Verified badge by hand
  unverify <username>      remove a hand-granted Verified badge

Flags:
  --role=admin|member      role for "create" (default admin)
  --password=<value>       password for "create" and "passwd"

The password may also be supplied in PHS_NEW_PASSWORD, or typed when prompted.
`

// runUserCLI executes a "user" subcommand and returns the process exit code.
func runUserCLI(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(cliUsage)
		return 0
	}

	command := args[0]
	rest, flags := splitFlags(args[1:])

	cfg, err := config.Load()
	if err != nil {
		return cliFail(err)
	}
	// The CLI talks to the operator, not to a log aggregator.
	log := logging.New(io.Discard, "error", "text")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return cliFail(err)
	}
	defer st.Close()
	_ = log

	name := func() (string, bool) {
		if len(rest) == 0 {
			fmt.Fprintf(os.Stderr, "error: %s needs a username\n", command)
			return "", false
		}
		return rest[0], true
	}

	switch command {
	case "list":
		return listUsers(ctx, st)

	case "create":
		username, ok := name()
		if !ok {
			return 2
		}
		role := flags["role"]
		if role == "" {
			role = store.RoleAdmin
		}
		password, err := resolvePassword(flags)
		if err != nil {
			return cliFail(err)
		}
		user, err := st.CreateUser(ctx, username, password, role)
		if err != nil {
			return cliFail(err)
		}
		fmt.Printf("created %s (%s)\n", user.Username, user.Role)
		return 0

	case "delete":
		username, ok := name()
		if !ok {
			return 2
		}
		if err := st.DeleteUser(ctx, username); err != nil {
			return cliFail(err)
		}
		fmt.Printf("deleted %s\n", username)
		return 0

	case "passwd":
		username, ok := name()
		if !ok {
			return 2
		}
		password, err := resolvePassword(flags)
		if err != nil {
			return cliFail(err)
		}
		if err := st.SetPassword(ctx, username, password); err != nil {
			return cliFail(err)
		}
		fmt.Printf("password updated for %s; all its sessions were revoked\n", username)
		return 0

	case "grant-admin", "revoke-admin":
		username, ok := name()
		if !ok {
			return 2
		}
		role := store.RoleAdmin
		if command == "revoke-admin" {
			role = store.RoleMember
		}
		user, err := st.SetRole(ctx, username, role)
		if err != nil {
			return cliFail(err)
		}
		fmt.Printf("%s is now %s\n", user.Username, user.Role)
		return 0

	case "set-owner":
		username, ok := name()
		if !ok {
			return 2
		}
		user, err := st.SetOwner(ctx, username)
		if err != nil {
			return cliFail(err)
		}
		fmt.Printf("%s is now the owner; the previous owner was demoted to admin\n", user.Username)
		return 0

	case "verify", "unverify":
		username, ok := name()
		if !ok {
			return 2
		}
		user, err := st.SetVerified(ctx, username, command == "verify", "cli")
		if err != nil {
			return cliFail(err)
		}
		if user.VerifiedAt != "" {
			fmt.Printf("%s is verified\n", user.Username)
		} else {
			fmt.Printf("%s is no longer manually verified\n", user.Username)
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", command, cliUsage)
		return 2
	}
}

func listUsers(ctx context.Context, st *store.Store) int {
	users, err := st.Users(ctx)
	if err != nil {
		return cliFail(err)
	}
	state, err := st.ProfileBadgeState(ctx)
	if err != nil {
		return cliFail(err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "USERNAME\tROLE\tPAGE\tBADGES\tCREATED")
	for _, u := range users {
		owns := state.OwnerID != 0 && u.ID == state.OwnerID
		labels := make([]string, 0, 2)
		for _, b := range st.UserBadges(u, owns, state.Views, state.Threshold) {
			labels = append(labels, b.Label)
		}
		if len(labels) == 0 {
			labels = append(labels, "-")
		}
		page := "-"
		if owns {
			page = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			u.Username, u.Role, page, strings.Join(labels, ", "), u.CreatedAt)
	}
	if err := tw.Flush(); err != nil {
		return cliFail(err)
	}

	fmt.Printf("\npage views: %d / %d towards automatic verification\n",
		state.Views, state.Threshold)
	return 0
}

// splitFlags separates --key=value flags from positional arguments.
func splitFlags(args []string) ([]string, map[string]string) {
	positional := make([]string, 0, len(args))
	flags := make(map[string]string, 2)

	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			positional = append(positional, arg)
			continue
		}
		key, value, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		flags[key] = value
	}
	return positional, flags
}

// resolvePassword takes the password from --password, then PHS_NEW_PASSWORD,
// then stdin. Note that a value passed on the command line is visible in the
// container's process list, so the environment variable is preferable.
func resolvePassword(flags map[string]string) (string, error) {
	if pw := flags["password"]; pw != "" {
		return pw, nil
	}
	if pw := os.Getenv("PHS_NEW_PASSWORD"); pw != "" {
		return pw, nil
	}

	fmt.Print("New password (input is visible): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		return "", errors.New("no password given")
	}
	return password, nil
}

func cliFail(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	return 1
}
