# Mail Checker

This project is a Go application that connects to an IMAP server, filters emails based on specified criteria, downloads attachments, and executes a command on them.

## Features

- Connects to an IMAP server using TLS.
- Filters emails by:
  - Subject
  - Sender
  - Seen/Unseen status
- Downloads attachments of a specific MIME type.
- Executes a shell command on the downloaded attachments.
- Configurable through environment variables.

## Configuration

The application is configured using the following environment variables:

| Variable          | Description                                     | Default           |
|-------------------|-------------------------------------------------|-------------------|
| `IMAP_SERVER`     | IMAP server address and port (e.g., `imap.example.com:993`) | **Required** |
| `EMAIL_USERNAME`  | Email account username                          | **Required**      |
| `EMAIL_PASSWORD`  | Email account password                          | **Required**      |
| `OUTPUT_DIR`      | Directory to save downloaded attachments        | `output`          |
| `IMAP_FOLDER`     | IMAP folder to check for emails                 | `INBOX`           |
| `SUBJECT_FILTER`  | Filter emails by subject (contains)             | ""                |
| `SENDER_FILTER`   | Filter emails by sender (contains)              | ""                |
| `UNSEEN_ONLY`     | Process only unread emails                      | `true`            |
| `READ_ONLY`       | Open the mailbox in read-only mode              | `true`            |
| `ATTACHMENT_TYPE` | MIME type of attachments to download            | `application/pdf` |
| `COMMAND`         | Command to execute on downloaded attachments    | `ls -lh %s`       |
| `LOG_LEVEL`       | Log level (DEBUG, INFO, WARN, ERROR)            | `INFO`            |

## Usage

### Build

To build the application, run:

```sh
make build
```

This will create an executable file in the `dist` directory.

### Run

To run the application, you can use the `run` command:

```sh
make run
```

Alternatively, you can run the compiled binary directly:

```sh
./dist/mail-checker
```

Make sure to set the required environment variables before running the application.

### Clean

To clean up the build files, run:

```sh
make clean
```

## Development

The `main.go` file contains the main application logic. It uses the `go-imap` library to interact with the IMAP server.

The `Makefile` provides convenient commands for building, running, and cleaning the project.
