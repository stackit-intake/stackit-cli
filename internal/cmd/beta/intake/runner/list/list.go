package list

import (
	"context"
	"fmt"

	"github.com/stackitcloud/stackit-cli/internal/pkg/types"

	"github.com/spf13/cobra"
	intake "github.com/stackitcloud/stackit-sdk-go/services/intake/v1betaapi"

	"github.com/stackitcloud/stackit-cli/internal/pkg/args"
	"github.com/stackitcloud/stackit-cli/internal/pkg/errors"
	"github.com/stackitcloud/stackit-cli/internal/pkg/examples"
	"github.com/stackitcloud/stackit-cli/internal/pkg/flags"
	"github.com/stackitcloud/stackit-cli/internal/pkg/globalflags"
	"github.com/stackitcloud/stackit-cli/internal/pkg/print"
	"github.com/stackitcloud/stackit-cli/internal/pkg/projectname"
	"github.com/stackitcloud/stackit-cli/internal/pkg/services/intake/client"
	"github.com/stackitcloud/stackit-cli/internal/pkg/tables"
)

const (
	limitFlag   = "limit"
	maxPageSize = 100
)

// inputModel struct holds all the input parameters for the command
type inputModel struct {
	*globalflags.GlobalFlagModel
	Limit *int64
}

// NewCmd creates a new cobra command for listing Intake Runners
func NewCmd(p *types.CmdParams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all Intake Runners",
		Long:  "Lists all Intake Runners for the current project.",
		Args:  args.NoArgs,
		Example: examples.Build(
			examples.NewExample(
				`List all Intake Runners`,
				`$ stackit beta intake runner list`),
			examples.NewExample(
				`List all Intake Runners in JSON format`,
				`$ stackit beta intake runner list --output-format json`),
			examples.NewExample(
				`List up to 5 Intake Runners`,
				`$ stackit beta intake runner list --limit 5`),
		),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			model, err := parseInput(p.Printer, cmd)
			if err != nil {
				return err
			}

			// Configure API client
			apiClient, err := client.ConfigureClient(p.Printer, p.CliVersion)
			if err != nil {
				return err
			}

			// Call API
			runners, err := fetchIntakeRunners(ctx, model, apiClient)
			if err != nil {
				return err
			}

			projectLabel := model.ProjectId
			if len(runners) == 0 {
				projectLabel, err = projectname.GetProjectName(ctx, p.Printer, p.CliVersion, cmd)
				if err != nil {
					p.Printer.Debug(print.ErrorLevel, "get project name: %v", err)
				}
			}

			return outputResult(p.Printer, model.OutputFormat, projectLabel, runners)
		},
	}
	configureFlags(cmd)
	return cmd
}

// configureFlags adds the --limit flag to the command
func configureFlags(cmd *cobra.Command) {
	cmd.Flags().Int64(limitFlag, 0, "Maximum number of entries to list")
}

// parseInput parses the command flags into a standardized model
func parseInput(p *print.Printer, cmd *cobra.Command) (*inputModel, error) {
	globalFlags := globalflags.Parse(p, cmd)
	if globalFlags.ProjectId == "" {
		return nil, &errors.ProjectIdError{}
	}

	limit := flags.FlagToInt64Pointer(p, cmd, limitFlag)
	if limit != nil && *limit < 1 {
		return nil, &errors.FlagValidationError{
			Flag:    limitFlag,
			Details: "must be greater than 0",
		}
	}

	model := inputModel{
		GlobalFlagModel: globalFlags,
		Limit:           limit,
	}

	p.DebugInputModel(model)
	return &model, nil
}

// buildRequest creates the API request to list Intake Runners
func buildRequest(ctx context.Context, model *inputModel, apiClient *intake.APIClient, pageToken *string, pageSize int32) intake.ApiListIntakeRunnersRequest {
	req := apiClient.DefaultAPI.ListIntakeRunners(ctx, model.ProjectId, model.Region)
	if pageToken != nil {
		req = req.PageToken(*pageToken)
	}
	req = req.PageSize(pageSize)
	return req
}

func fetchIntakeRunners(ctx context.Context, model *inputModel, apiClient *intake.APIClient) ([]intake.IntakeRunnerResponse, error) {
	var pageToken *string
	var runners []intake.IntakeRunnerResponse
	for {
		pageSize := int32(maxPageSize)
		if model.Limit != nil {
			remaining := *model.Limit - int64(len(runners))
			if remaining <= 0 {
				break
			}
			if remaining < int64(pageSize) {
				pageSize = int32(remaining)
			}
		}

		req := buildRequest(ctx, model, apiClient, pageToken, pageSize)
		resp, err := req.Execute()
		if err != nil {
			return nil, fmt.Errorf("list Intake Runners: %w", err)
		}
		respRunners := resp.GetIntakeRunners()
		if len(respRunners) == 0 {
			break
		}
		runners = append(runners, respRunners...)

		if model.Limit != nil && len(runners) >= int(*model.Limit) {
			runners = runners[:*model.Limit]
			break
		}

		if !resp.HasNextPageToken() || resp.GetNextPageToken() == "" {
			break
		}
		token := resp.GetNextPageToken()
		pageToken = &token
	}
	return runners, nil
}

// outputResult formats the API response and prints it to the console
func outputResult(p *print.Printer, outputFormat, projectLabel string, runners []intake.IntakeRunnerResponse) error {
	return p.OutputResult(outputFormat, runners, func() error {
		if len(runners) == 0 {
			p.Outputf("No intake runners found for project %q\n", projectLabel)
			return nil
		}

		table := tables.NewTable()

		table.SetHeader("ID", "NAME", "STATE")
		for _, runner := range runners {
			table.AddRow(
				runner.GetId(),
				runner.GetDisplayName(),
				runner.GetState(),
			)
		}
		err := table.Display(p)
		if err != nil {
			return fmt.Errorf("render table: %w", err)
		}
		return nil
	})
}
