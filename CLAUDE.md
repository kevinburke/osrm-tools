Please refrain from being overly agreeable, complimentary, or apologetic. This
can distort the truth, inhibit my own personal growth, and lead us down paths
that aren't the most useful. If you have a good idea you're confident in, don't
discard it just because I suggest something different. Where possible try to
consider our relative levels of expertise and confidence in evaluating ideas.

It's really important to *avoid failing silently*. This can lead to various
undesired behaviors, most importantly, thinking the program succeeded when it
actually failed. So let's e.g. run "set -e" in Bash and other tools that will
help us make clear when a program did not work.

Let's also try to make it hard for people to make mistakes. If we make a mistake
- take a moment to see how we could make it so no one could ever make that
mistake again. If we fix an error/warning, take a moment to see where else that
error could pop up.

We also want to treat warnings as errors, where possible.

If you are writing a Go command:

- Make sure to call `flag.Parse()` to ensure that if the user writes `--help`
  that we print out some output about how to use the command.

- If this is designed to be a long lived tool, let's also add a `--version`
  flag.

- Prefer log/slog to using e.g. fmt.Print

- When we run 'go' subcommands (build, test, etc) prefer including the
  `-trimpath` flag.

- Proper formatting is important so a good practice is to run `go fix ./...`,
`go fmt ./...` and `goimports -w .` before returning control to me.

- If you are compiling a go binary, or verifying if something builds, please
don't leave it in the Git root of the project. Create a temp directory, and
compile there.

    ```go
    mkdir -p tmp
    go build -trimpath -o tmp/ ./cmd/mybinary
    ```

If you are invoking a command, prefer using the long argument names to the short
ones - so for curl, use e.g. --remote-name instead of -O.

## Repos and projects

Any code you want to retrieve from Github, please try looking in
~/src/github.com/owner/repo first. It is likely checked out to disk and it is
easier to search from there.
