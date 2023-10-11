package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/alecthomas/kong"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/charmbracelet/log"
	"github.com/google/go-github/v55/github"
	"github.com/newrelic/go-agent/v3/newrelic"

	"github.com/shakefu/gha-debug/pkg/fileflag"
)

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

// Parse returns a new Cli instance from passed arguments
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
	// TODO: There's a bug where if these have defaults they try to read the file, even if this command is not being used...
	// Need to file an issue about that and get it fixed
	NewRelicSecret       kong.NamedFileContentFlag `short:"n" type:"namedfilecontent" help:"Path to New Relic License Key secret."`
	GHAppIDSecret        kong.NamedFileContentFlag `short:"a" type:"namedfilecontent" help:"Path to GitHub App ID secret."`
	GHAppInstallIDSecret kong.NamedFileContentFlag `short:"i" type:"namedfilecontent" help:"Path to GitHub App Installation ID secret."`
	GHAppPrivateKey      string                    `short:"k" type:"existingfile" help:"Path to GitHub App Private Key secret."`
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
	// Useless over-debugging
	log.Debug("Repo", "repo", start.Repo)
	log.Debug("Workflow", "workflow", start.Workflow)
	log.Debug("Job", "job", start.Job)
	log.Debug("Branch", "branch", start.Branch)

	log.Debug("NewRelicSecret", "secret", string(start.NewRelicSecret.Contents))
	log.Debug("GHAppIDSecret", "secret", string(start.GHAppIDSecret.Contents))
	log.Debug("GHAppInstallIDSecret", "secret", string(start.GHAppInstallIDSecret.Contents))
	log.Debug("GHAppPrivateKey", "secret", string(start.GHAppPrivateKey.Contents))

	log.Debug("GITHUB_RUN_ID", "env", os.Getenv("GITHUB_RUN_ID"))
	log.Debug("RUNNER_NAME", "env", os.Getenv("RUNNER_NAME")
	**/

	// Get the NewRelic App instance from our CLI params
	app, err := start.NewRelicApp()
	if err != nil {
		log.Fatal("Could not create NewRelic app", "err", err)
		return
	}

	// NewRelic transaction name is the workflow name and job name
	txnName := fmt.Sprintf("%s / %s", start.Workflow, start.Job)

	// Create a FileFlag semaphore to listen for the flag file
	flag, err := fileflag.NewFileFlag(cli.Flag)
	if err != nil {
		log.Fatal("Could not create flag file", "err", err)
		return
	}

	// Start watching for file events
	go flag.Watch()
	runtime.Gosched()

	// Create the flag file if it doesn't exist
	err = touchFile(cli.Flag)
	if err != nil {
		log.Fatal("Could not create flag file", "err", err)
		return
	}

	// Wait for the start flag
	log.Debug("Waiting for watcher start")
	flag.WaitForStart()

	// Start a new transaction
	txn := app.StartTransaction(txnName)

	// Annotate the with attributes
	txn.AddAttribute("branch", start.Branch)
	txn.AddAttribute("workflow", start.Workflow)
	txn.AddAttribute("job", start.Job)
	txn.AddAttribute("repo", start.Repo)
	txn.AddAttribute("runner", os.Getenv("RUNNER_NAME"))
	txn.AddAttribute("actor", os.Getenv("GITHUB_ACTOR"))
	txn.AddAttribute("triggering_actor", os.Getenv("GITHUB_TRIGGERING_ACTOR"))
	txn.AddAttribute("run_number", os.Getenv("GITHUB_RUN_NUMBER"))
	txn.AddAttribute("run_id", os.Getenv("GITHUB_RUN_ID"))

	// URL format
	// https://github.com/turo/github-actions-scale-set-deployments/actions/runs/6322221331
	txn.AddAttribute("run_url", fmt.Sprintf("https://github.com/%s/actions/runs/%s", start.Repo, os.Getenv("GITHUB_RUN_ID")))

	// Waiting on our flag to be removed, indicating all the jobs are done
	log.Info("Waiting...")
	flag.Wait()

	// Get the Job status
	status, err := start.GitHubJobStatus()
	txn.AddAttribute("status", status)
	if err != nil {
		log.Warn("Could not get Job status", "err", err)
	}

	// End the transaction
	txn.End()
	flag.Close()
	log.Info("Done.")

	// Default to 60s timeout sending data to NR
	log.Debug("Sending data to NewRelic...")
	app.Shutdown(60 * time.Second)

	log.Debug("Shutdown complete.")

	return
}

// structToJSON is a helper for pretty printing structs (mostly used for GH API responses/objects)
func structToJSON(data interface{}) (out string) {
	j, _ := json.MarshalIndent(data, "", "  ")
	out = string(j)
	return
}

// touchFile is a helper to create an empty file at the given path for use as a flag file
func touchFile(path string) (err error) {
	// Ensure the directory exists
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return
	}
	// Create the file
	_, err = os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		_, err = os.Create(path)
	}
	return
}

// GitHubClient returns a GitHub client instance ready to use
func (start *CliStart) GitHubClient() (client *github.Client, err error) {
	// Parse int appID out of our byte file content
	appID, err := strconv.ParseInt(strings.TrimSpace(string(start.GHAppIDSecret.Contents)), 10, 64)
	if err != nil {
		return
	}

	// Parse int appInstID out of our byte file content
	appInstID, err := strconv.ParseInt(strings.TrimSpace(string(start.GHAppInstallIDSecret.Contents)), 10, 64)
	if err != nil {
		return
	}

	appKey := start.GHAppPrivateKey

	// Wrap the shared transport for use with the app ID 1 authenticating with
	// installation ID 99.
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

// GitHubJobStatus returns the status of the current job from the GitHub API if
// we can find it.
func (start *CliStart) GitHubJobStatus() (status string, err error) {
	// Default status to "unknown"
	status = "unknown"

	// Use the GitHub client to retrieve run information
	ghRunID := os.Getenv("GITHUB_RUN_ID")
	if ghRunID == "" {
		log.Warn("Could not get GITHUB_RUN_ID")
		return
	}

	// API client wants a 64-bit int
	runID, err := strconv.ParseInt(ghRunID, 10, 64)
	if err != nil {
		log.Warn("Could not parse GITHUB_RUN_ID", "err", err)
		// TODO: Figure out if we want this to error harder
		err = nil
		return
	}

	// Split the org and repo name from the repo string, since the API wants
	// them separate
	orgName, repoName, found := strings.Cut(start.Repo, "/")
	if !found {
		log.Warn("Could not parse GITHUB_REPOSITORY", "repo", start.Repo)
		return
	}

	// Runner name is unique with Ephemeral runners, so we can use it to find
	// our job since we don't have the Job ID in our environment
	runnerName := os.Getenv("RUNNER_NAME")
	if runnerName == "" {
		log.Warn("Could not get RUNNER_NAME")
		return
	}

	// Get the GitHub client instance from our CLI params
	client, err := start.GitHubClient()
	if err != nil {
		log.Warn("Could not create GitHub client", "err", err)
		// TODO: Figure out if we want this to error harder
		err = nil
		return
	}

	// Context for calling the API with a timeout of 30s
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Call the API to get the Jobs associated with the workflow run
	run, response, err := client.Actions.ListWorkflowJobs(ctx, orgName, repoName, runID, &github.ListWorkflowJobsOptions{Filter: "all"})
	if err != nil {
		return
	}

	// Sanity check
	if response.Rate.Remaining < 2 {
		log.Warn("GitHub API rate limit exceeded", "rate", structToJSON(response.Rate))
	}

	// Iterate through all the jobs looking for our runner name, which
	// identifies this current run uniquely
	var job *github.WorkflowJob
	for _, item := range run.Jobs {
		if *item.RunnerName == runnerName {
			job = item
			break
		}
	}
	if job == nil {
		log.Warn("Could not find Job matching RUNNER_NAME", "runnerName", runnerName)
		return
	}

	// Iterate through all the steps in our job, checking their conclusion
	status = "success"
	for _, step := range job.Steps {
		var conclusion string
		if step.Conclusion != nil {
			conclusion = *step.Conclusion
		} else {
			conclusion = "unknown"
		}
		if conclusion == "failure" {
			status = "failure"
			// Break out of the loop, since we consider one failure to be the
			// entire job failing for now
			// TODO: Figure out if there's a way to detect a failing step that
			// isn't failing the whole Job (before the Job status is reported,
			// which it won't be in this case)
			break
		}
	}

	log.Info("Job status", "status", status)
	return
}

// NewRelicApp returns a NewRelic app instance ready to use
func (start *CliStart) NewRelicApp() (app *newrelic.Application, err error) {
	// Parse the license key out of our byte file content
	licenseKey := strings.TrimSpace(string(start.NewRelicSecret.Contents))
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
		log.Debug("Debug output enabled")
	}

	// TODO: Decide if we want to JSON format logs
	// log.SetFormatter(log.JSONFormatter)

	err := cli.Main()
	if err != nil {
		log.Fatal("Error", "err", err)
	}
}
