package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/alecthomas/kong"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/charmbracelet/log"
	"github.com/google/go-github/v55/github"
	"github.com/newrelic/go-agent/v3/newrelic"

	. "github.com/shakefu/gha-debug/pkg/softlock"
)

// AppTransaction represents a single transaction to be monitored by NewRelic
type AppTransaction struct {
	app       *newrelic.Application // NewRelic application instance
	txn       *newrelic.Transaction // NewRelic transaction instance
	lock      *SoftLock             // Our shared lock for the transaction
	startFile string                // Filename to read for starting context
	endFile   string                // Filename to read for ending context
	workflow  string                // Workflow name
	job       string                // Job name
}

// NewTransaction creates a new AppTransaction instance and initializes the NewRelic app
func NewTransaction(newRelicApp string, newRelicLicense string, lock *SoftLock, startFile string, endFile string) *AppTransaction {
	// Create new NR app
	var app *newrelic.Application
	var err error

	// Application is GITHUB_REPOSITORY "turo/github-actions-runner-deployemtns"
	// AppTransaction Name is GITHUB_WORKFLOW + GITHUB_JOB (+ branch name???)
	// "GHA Scale Set / gha-scale-set-test-secondary"
	// No segments
	// Attributes:
	//   branch (GITHUB_HEAD_REF)
	//   URL (availble via the API call OR use the `/runs/{run_id}` endpoint)
	//   run attempt (GITHUB_RUN_NUMBER)
	//   actor (GITHUB_ACTOR)
	//   triggering actor (GITHUB_TRIGGERING_ACTOR)

	// Mock out the NR App if we don't have a license
	if newRelicApp == "" || newRelicLicense == "" {
		// This is nil-safe/the correct mocking behavior according to:
		// https://pkg.go.dev/github.com/newrelic/go-agent/v3@v3.24.1/newrelic#Application
		app = nil
		// app = &newrelic.Application{}
	} else {
		appName := newrelic.ConfigAppName(newRelicApp)
		license := newrelic.ConfigLicense(newRelicLicense)
		// Create the NR Application for this transaction
		app, err = newrelic.NewApplication(appName, license)
		if err != nil {
			// This is a hard failure, we can't continue, so we panic
			log.Fatal(err)
		}
	}
	t := &AppTransaction{
		app:       app,
		txn:       nil,
		lock:      lock,
		startFile: startFile,
		endFile:   endFile,
	}
	return t
}

// Monitor function - this is where we do the work
func (t *AppTransaction) Monitor() {
	// Ensure we always end nicely
	defer t.Cleanup()

	log.Info("Action started")

	// Parse our context file so we can reference our names correctly
	// t.ParseContext(t.startFile)

	// Create the transaction name based on workflow and job
	transaction := fmt.Sprintf("%s / %s", t.workflow, t.job)
	t.txn = t.app.StartTransaction(transaction)

	// Hang out here until we're finished
	t.lock.Wait()

	// Parse the end file
	// TODO: Figure out if there's extra info in here that we actually want
	// TODO: Figure out if we can get success/fail status without calling the API
	// t.ParseContext(t.endFile)

	log.Info("Action finished")
}

/*
// ParseContext parses the GitHub Action context file JSON
func (t *AppTransaction) ParseContext(filename string) {
	var err error
	var data []byte

	data, err = os.ReadFile(filename)
	if err != nil {
		log.Fatal("Could not read context file", "err", err)
	}
	log.Debug("Context file", "context", string(data))

	var context map[string]interface{}
	err = json.Unmarshal(data, &context)
	if err != nil {
		log.Fatal("Could not parse context file", "err", err)
	}

	// TODO: Pull the workflow name and job name from the context
}
*/

// Cleanup gives us a way to reliably end the transaction and clean up
func (t *AppTransaction) Cleanup() {
	// Ensure the lock is fully released and the program can exit fully, even if
	// something goofy happens with the NR calls below
	defer t.lock.Close()

	// End the NR transaction
	t.txn.End()

	// Default to 60s timeout sending data to NR
	t.app.Shutdown(60 * time.Second)
}

type ActionMonitor struct {
}

func NewActionMonitor(client *github.Client, app *newrelic.Application, txnName string) (monitor *ActionMonitor) {
	return
}

func (monitor *ActionMonitor) Start() {
	// Do nothing?
}

/*
 * Main CLI
 */

// Cli declares our Kong CLI options so we can extend the type with a few helper functions
type Cli struct {
	Debug bool `short:"d" help:"Debug mode."`

	Start CliStart `cmd:"" help:"Start the process and open a new transaction." default:"withargs"`
	Stop  CliStop  `cmd:"" help:"Stop a currently waiting transaction and send data to NewRelic, exiting the process."`

	// More options
	Flag string `short:"f" type:"path" default:"./gha-debug.flag" help:"Flag file to watch for starting and stopping the transaction."`

	// Kong context object
	ctx *kong.Context `kong:"-"`
}

// Parse returns a new Cli instance
func (cli *Cli) Parse() {
	cli.ctx = kong.Parse(cli,
		kong.Name("gha-debug"),
		kong.Description("A GitHub Actions debug tool."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}))
}

// Main runs the command specified
func (cli *Cli) Main() error {
	log.Debug("Running", "command", cli.ctx.Command())

	return cli.ctx.Run(cli)
}

/**
// Run in theory will run after command specific Run calls are made, but that's
// not useful for us here.
func (cli *Cli) Run() (err error) {
	log.Debug("Run command")
	return
}
*/

/*
 * Start subcommand
 *
 * This will start the process and open a new transaction in NewRelic. It will
 * also optionally create the flag file if it doesn't exist. It will attempt to
 * read the information given by the GitHub Actions Runner process to determine
 * the repository, workflow name, job ID, and branch name.
 *
 * When the flag file is removed, it will send the collected data to NewRelic
 * and exit.
 */

// CliStart is the 'start' subcommand
type CliStart struct {
	// TODO: Optional flag for creating the flag file if it doesn't exist?

	// GitHub Job context variables (supplied by runner process)
	Repo     string `short:"r" type:"string" required:"" env:"GITHUB_REPOSITORY" placeholder:"REPOSITORY" help:"GitHub repository."`
	Workflow string `short:"w" type:"string" required:"" env:"GITHUB_WORKFLOW" placeholder:"WORKFLOW" help:"GitHub workflow."`
	Job      string `short:"j" type:"string" required:"" env:"GITHUB_JOB" placeholder:"JOB" help:"GitHub job ID."`
	Branch   string `short:"b" type:"string" required:"" env:"GITHUB_HEAD_REF" placeholder:"BRANCH" help:"GitHub branch."`

	// Required secrets for talking to GH and NR Apis
	NewRelicSecret       kong.NamedFileContentFlag `short:"n" type:"namedfilecontent" default:"./new_relic_license_key" help:"Path to New Relic License Key secret."`
	GHAppIDSecret        kong.NamedFileContentFlag `short:"a" type:"namedfilecontent" default:"./github_app_id" help:"Path to GitHub App ID secret."`
	GHAppInstallIDSecret kong.NamedFileContentFlag `short:"i" type:"namedfilecontent" default:"./github_app_installation_id" help:"Path to GitHub App Installation ID secret."`
	GHAppPrivateKey      string                    `short:"k" type:"existingfile" default:"./github_app_private_key" help:"Path to GitHub App Private Key secret."`
}

// Help returns the help text for the "start" command
func (start *CliStart) Help() string {
	return heredoc.Doc(`
	This command will start the process and open a new transaction in NewRelic.
	It will attempt to read the information given by the GitHub Actions Runner
	process to determine the repository, workflow name, job ID, and branch name.
	`)
}

// Run executes the "start" command
func (start *CliStart) Run(cli *Cli) (err error) {
	log.Debug("Start command")

	/**
	// Useless over- debugging
	log.Debug("Repo", "repo", start.Repo)
	log.Debug("Workflow", "workflow", start.Workflow)
	log.Debug("Job", "job", start.Job)
	log.Debug("Branch", "branch", start.Branch)

	log.Debug("NewRelicSecret", "secret", string(start.NewRelicSecret.Contents))
	log.Debug("GHAppIDSecret", "secret", string(start.GHAppIDSecret.Contents))
	log.Debug("GHAppInstallIDSecret", "secret", string(start.GHAppInstallIDSecret.Contents))
	log.Debug("GHAppPrivateKey", "secret", string(start.GHAppPrivateKey.Contents))
	**/

	// Get the GitHub client instance from our CLI params
	client, err := start.GitHubClient()
	if err != nil {
		log.Fatal("Could not create GitHub client", "err", err)
		return
	}

	// Get the NewRelic App instance from our CLI params
	app, err := start.NewRelicApp()

	// NewRelic transaction name is the workflow name and job name
	txnName := fmt.Sprintf("%s / %s", start.Workflow, start.Job)

	// Create a new ActionMonitor
	monitor := NewActionMonitor(client, app, txnName)
	monitor.Start()

	// TODO: Annotate the with attributes
	//   branch (GITHUB_HEAD_REF)
	//   URL (availble via the API call OR use the `/runs/{run_id}` endpoint)
	//   run attempt (GITHUB_RUN_NUMBER)
	//   actor (GITHUB_ACTOR)
	//   triggering actor (GITHUB_TRIGGERING_ACTOR)

	return
}

// GitHubClient returns a GitHub client instance ready to use
func (start *CliStart) GitHubClient() (client *github.Client, err error) {
	// Parse int appID out of our byte file content
	appID, err := strconv.ParseInt(string(start.GHAppIDSecret.Contents), 10, 64)
	if err != nil {
		return
	}

	// Parse int appInstID out of our byte file content
	appInstID, err := strconv.ParseInt(string(start.GHAppInstallIDSecret.Contents), 10, 64)
	if err != nil {
		return
	}

	appKey := start.GHAppPrivateKey

	// Wrap the shared transport for use with the app ID 1 authenticating with installation ID 99.
	itr, err := ghinstallation.NewKeyFromFile(
		http.DefaultTransport,
		appID,
		appInstID,
		appKey,
	)

	// Create the GitHub client
	client = github.NewClient(&http.Client{Transport: itr})
	return
}

// NewRelicApp returns a NewRelic app instance ready to use
func (start *CliStart) NewRelicApp() (app *newrelic.Application, err error) {
	// Parse the license key out of our byte file content
	licenseKey := string(start.NewRelicSecret.Contents)
	// Application name is the repo name
	appName := start.Repo

	// Create the NR Application for this transaction
	app, err = newrelic.NewApplication(
		newrelic.ConfigLicense(licenseKey),
		newrelic.ConfigAppName(appName),
	)
	return
}

/*
 * Stop subcommand
 *
 * This command just removes the flag file, which will cause the process which
 * is listening for it to send its data to NewRelic and exit.
 */

// CliStop is the 'stop' subcommand
type CliStop struct{}

// Help for the "stop" command
func (stop *CliStop) Help() string {
	return heredoc.Doc(`
	TODO: More help here, if needed.
	`)
}

// Run executes the "stop" command
func (stop *CliStop) Run(cli *Cli) (err error) {
	log.Info("Stopping transaction...")
	filename := cli.Flag
	// Check if the path at cli.Flag exists and remove it if it does
	if _, err = os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		// file does not exist
		log.Debug("Flag file does not exist, nothing happened")
	} else if err != nil {
		log.Error("Error", "err", err)
	} else {
		// file exists
		log.Debug("Flag file exists, cleaning", "filename", filename)
		err = os.Remove(filename)
	}
	return
}

// main runs things
func main() {
	var cli Cli
	cli.Parse()

	if cli.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode enabled")
	}

	// TODO: Decide if we want to JSON format logs
	// log.SetFormatter(log.JSONFormatter)

	cli.Main()

	// Debugging
	if true {
		return
	}

	// Create a new transaction
	// TODO: Pass in workflow info/figure out if we want to use a shared struct to pass this around
	// TODO: Make the file semaphore actually listen for a file to be created and then removed
	// TODO: Integrate this with SoftLock to just create a full file semaphore option?
	// Start listening for FS events.
}
