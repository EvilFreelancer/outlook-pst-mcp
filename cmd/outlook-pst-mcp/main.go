package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"

	"outlook-pst-mcp/internal/app"
	"outlook-pst-mcp/internal/mcpserver"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "import" {
		importCmd(os.Args[2:])
		return
	}
	serveCmd(os.Args[1:])
}

func serveCmd(args []string) {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	workspace := flags.String("workspace", ".", "workspace directory for mailbox state")
	_ = flags.Parse(args)

	server := mcpserver.NewLazy(*workspace)
	defer func() {
		if err := server.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	stdout := bufio.NewWriter(os.Stdout)
	if err := server.Serve(context.Background(), os.Stdin, stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func importCmd(args []string) {
	flags := flag.NewFlagSet("import", flag.ExitOnError)
	workspace := flags.String("workspace", "", "workspace directory for mailbox state")
	pstPath := flags.String("pst", "", "path to PST file")
	_ = flags.Parse(args)

	if *workspace == "" || *pstPath == "" {
		fmt.Fprintln(os.Stderr, "usage: outlook-pst-mcp import -workspace <dir> -pst <file.pst>")
		os.Exit(2)
	}

	service, err := app.Open(*workspace)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer service.Close()

	folders, messages, skipped, err := service.ImportMailbox(*pstPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("{\"folder_count\":%d,\"message_count\":%d,\"skipped_count\":%d,\"extracted_count\":%d}\n", folders, messages, skipped, messages+skipped)
}
