package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
)

var (
	imapServer      = os.Getenv("IMAP_SERVER")
	username        = os.Getenv("EMAIL_USERNAME")
	password        = os.Getenv("EMAIL_PASSWORD")
	outputDir       = cmp.Or(os.Getenv("OUTPUT_DIR"), "output")
	imapFolder      = cmp.Or(os.Getenv("IMAP_FOLDER"), "INBOX")
	subjectFilter   = cmp.Or(os.Getenv("SUBJECT_FILTER"), "")
	senderFilter    = cmp.Or(os.Getenv("SENDER_FILTER"), "")
	unSeenOnly      = cmp.Or(os.Getenv("UNSEEN_ONLY"), "true") == "true"
	isReadOnly      = cmp.Or(os.Getenv("READ_ONLY"), "true") == "true"
	attachmentTypes = strings.Split(cmp.Or(os.Getenv("ATTACHMENT_TYPES"), "application/pdf"), ",")
	command         = cmp.Or(os.Getenv("COMMAND"), "stat '%s'")

	ErrMissingVar        = errors.New("IMAP_SERVER, EMAIL_USERNAME and EMAIL_PASSWORD environment variables must be set")
	ErrCreatingOutputDir = errors.New("error creating output dir")
	ErrNoMsgs            = errors.New("no messages found in the mailbox")

	LogLevel = levelByName(cmp.Or(os.Getenv("LOG_LEVEL"), "INFO"))
)

func main() {
	logger := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevel})
	slog.SetDefault(slog.New(logger))

	if imapServer == "" || username == "" || password == "" {
		slog.Error(ErrMissingVar.Error())
		os.Exit(1)
	}

	if err := os.MkdirAll(outputDir, 0700); err != nil {
		slog.Error(ErrCreatingOutputDir.Error())
		os.Exit(1)
	}

	c, err := newClient(imapServer, username, password)
	if err != nil {
		if errors.Is(err, ErrNoMsgs) {
			slog.Info(err.Error())
			os.Exit(0)
		}

		slog.Error("Failed with IMAP client", "error", err)
		os.Exit(1)
	}
	defer func() { _ = c.Logout() }()

	seqset := createFilter(c)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 10)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchRFC822, imap.FetchBody}, messages)
	}()

	i := 1
	total := int(c.Mailbox().Messages)
	for msg := range messages {
		slog.Debug("Processing message", "seq", i, "subject", msg.Envelope.Subject, "date", msg.Envelope.Date)
		if err := processMessage(msg); err != nil {
			slog.Error(err.Error())
			break
		}

		if LogLevel > slog.LevelDebug {
			progress(i, total)
		}
		i++
	}
	fmt.Println("\nProcessing complete")

	if err = <-done; err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func newClient(server, username, password string) (*client.Client, error) {
	c, err := client.DialTLS(server, nil)
	if err != nil {
		return nil, err
	}
	if err := c.Login(username, password); err != nil {
		_ = c.Logout()
		return nil, err
	}

	mbox, err := c.Select(imapFolder, isReadOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	if mbox.Messages == 0 {
		return nil, ErrNoMsgs
	}

	slog.Debug("Connected to mailbox", "folder", mbox.Name, "messages", mbox.Messages)

	return c, nil
}

func createFilter(c *client.Client) *imap.SeqSet {
	criteria := imap.NewSearchCriteria()
	if unSeenOnly {
		criteria.WithoutFlags = []string{"\\Seen"}
	}

	if subjectFilter != "" {
		criteria.Header.Set("Subject", subjectFilter)
	}

	if senderFilter != "" {
		criteria.Header.Set("From", senderFilter)
	}

	uids, err := c.Search(criteria)
	if err != nil {
		slog.Error("Failed to search for messages", "error", err)
		os.Exit(1)
	}

	if len(uids) == 0 {
		slog.Info("No unread messages found")
		os.Exit(0)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)
	slog.Debug(fmt.Sprintf("Processing %d filtered messages", len(uids)))

	return seqset
}

func processMessage(msg *imap.Message) error {
	for _, r := range msg.Body {
		entity, err := message.Read(r)
		if err != nil {
			return fmt.Errorf("failed to read message body: %w", err)
		}

		mpr := entity.MultipartReader()
		if mpr == nil {
			if err = processAttachment(entity, msg.Envelope.Date); err != nil {
				return fmt.Errorf("failed to processing message: %w", err)
			}
			continue
		}

		for e, err := mpr.NextPart(); err != io.EOF; e, err = mpr.NextPart() {
			if err != nil {
				slog.Error("failed to read multipart section", "error", err)
				continue
			}

			if err := processAttachment(e, msg.Envelope.Date); err != nil {
				slog.Error("failed to process attachment", "error", err)
				continue
			}
		}
	}

	return nil
}

func processAttachment(e *message.Entity, date time.Time) error {
	kind, params, cErr := e.Header.ContentType()
	if cErr != nil {
		return fmt.Errorf("failed to get content type: %w", cErr)
	}

	if slices.Index(attachmentTypes, kind) == -1 {
		slog.Debug("skipping part with content", "type", kind, "expected", attachmentTypes)
		return nil
	}

	if params["name"] == "" {
		_, p, err := e.Header.ContentDisposition()
		if err != nil {
			return errors.New("skipping unrecognized attachment")
		}
		params["name"] = p["filename"]
	}

	filename := filepath.Clean(filepath.Join(outputDir, params["name"]))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	if _, err = io.Copy(file, e.Body); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	if err = file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	_ = os.Chtimes(filename, date, date)

	slog.Info("Saved attachment", "name", params["name"])

	if command != "" {
		return execCommand(filename)
	}

	return nil
}

func execCommand(filename string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf(command, filename))

	if LogLevel > slog.LevelDebug {
		slog.Info("Command executed successfully")
		return cmd.Run()
	}

	out, _ := os.Create(filepath.Clean(filename + ".out"))
	defer func() { _ = out.Close() }()

	output, err := cmd.CombinedOutput()
	if err != nil {
		_, _ = out.Write(output)
		return fmt.Errorf("command execution failed: %w", err)
	}

	if _, err = out.Write(output); err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Command executed successfully and output written to %q", filename+".out"))

	return nil
}

func levelByName(name string) slog.Level {
	switch strings.ToUpper(name) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return 0
	}
}
