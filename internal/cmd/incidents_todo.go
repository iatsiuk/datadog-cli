package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var (
	errTodoDescriptionRequired = errors.New("--description is required")
	errTodoYesRequired         = errors.New("--yes is required to delete a todo")
)

func newIncidentsTodoCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Manage incident todos",
	}
	cmd.AddCommand(newTodoListCmd(mkAPI))
	cmd.AddCommand(newTodoShowCmd(mkAPI))
	cmd.AddCommand(newTodoCreateCmd(mkAPI))
	cmd.AddCommand(newTodoUpdateCmd(mkAPI))
	cmd.AddCommand(newTodoDeleteCmd(mkAPI))
	return cmd
}

func todoTableRows(todos []datadogV2.IncidentTodoResponseData) [][]string {
	rows := make([][]string, 0, len(todos))
	for _, t := range todos {
		attrs := t.GetAttributes()
		assignees := todoAssigneesString(attrs.GetAssignees())
		completed := ""
		if c, ok := attrs.GetCompletedOk(); ok && c != nil && *c != "" {
			completed = *c
		}
		rows = append(rows, []string{t.GetId(), attrs.GetContent(), assignees, completed})
	}
	return rows
}

func todoAssigneesString(assignees []datadogV2.IncidentTodoAssignee) string {
	handles := make([]string, 0, len(assignees))
	for _, a := range assignees {
		if a.IncidentTodoAssigneeHandle != nil {
			handles = append(handles, *a.IncidentTodoAssigneeHandle)
		}
	}
	return strings.Join(handles, ", ")
}

func printTodoDetail(cmd *cobra.Command, d datadogV2.IncidentTodoResponseData) error {
	attrs := d.GetAttributes()
	rows := [][]string{
		{"ID", d.GetId()},
		{"CONTENT", attrs.GetContent()},
		{"ASSIGNEES", todoAssigneesString(attrs.GetAssignees())},
	}
	if c, ok := attrs.GetCompletedOk(); ok && c != nil && *c != "" {
		rows = append(rows, []string{"COMPLETED", *c})
	}
	if t := attrs.GetCreated(); !t.IsZero() {
		rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newTodoListCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list <incident-id>",
		Short: "List todos for an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.ListIncidentTodos(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list incident todos: %w", err)
			}

			todos := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if todos == nil {
					todos = []datadogV2.IncidentTodoResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), todos)
			}

			headers := []string{"ID", "CONTENT", "ASSIGNEES", "COMPLETED"}
			return output.PrintTable(cmd.OutOrStdout(), headers, todoTableRows(todos))
		},
	}
}

func newTodoShowCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <incident-id> <todo-id>",
		Short: "Show todo details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.GetIncidentTodo(iapi.ctx, args[0], args[1])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident todo: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printTodoDetail(cmd, d)
		},
	}
}

func newTodoCreateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var (
		description string
		assignee    string
	)

	cmd := &cobra.Command{
		Use:   "create <incident-id>",
		Short: "Create a todo for an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if description == "" {
				return errTodoDescriptionRequired
			}

			assignees := []datadogV2.IncidentTodoAssignee{}
			if assignee != "" {
				h := assignee
				assignees = append(assignees, datadogV2.IncidentTodoAssigneeHandleAsIncidentTodoAssignee(&h))
			}

			attrs := datadogV2.NewIncidentTodoAttributes(assignees, description)
			data := datadogV2.NewIncidentTodoCreateData(*attrs, datadogV2.INCIDENTTODOTYPE_INCIDENT_TODOS)
			body := datadogV2.NewIncidentTodoCreateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.CreateIncidentTodo(iapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create incident todo: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printTodoDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "todo content (required)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "assignee handle")
	return cmd
}

func newTodoUpdateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var (
		description string
		completed   bool
	)

	cmd := &cobra.Command{
		Use:   "update <incident-id> <todo-id>",
		Short: "Update an incident todo",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			// fetch current to preserve required fields
			getResp, httpResp, err := iapi.api.GetIncidentTodo(iapi.ctx, args[0], args[1])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident todo: %w", err)
			}
			current := getResp.GetData()
			currentAttrs := current.GetAttributes()

			attrs := datadogV2.NewIncidentTodoAttributes(
				currentAttrs.GetAssignees(),
				currentAttrs.GetContent(),
			)

			if cmd.Flags().Changed("description") {
				attrs.SetContent(description)
			}
			if cmd.Flags().Changed("completed") {
				if completed {
					attrs.SetCompleted("true")
				} else {
					attrs.SetCompletedNil()
				}
			}

			data := datadogV2.NewIncidentTodoPatchData(*attrs, datadogV2.INCIDENTTODOTYPE_INCIDENT_TODOS)
			body := datadogV2.NewIncidentTodoPatchRequest(*data)

			resp, httpResp2, err := iapi.api.UpdateIncidentTodo(iapi.ctx, args[0], args[1], *body)
			if httpResp2 != nil {
				_ = httpResp2.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update incident todo: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printTodoDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "new todo content")
	cmd.Flags().BoolVar(&completed, "completed", false, "mark as completed")
	return cmd
}

func newTodoDeleteCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <incident-id> <todo-id>",
		Short: "Delete an incident todo",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return errTodoYesRequired
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := iapi.api.DeleteIncidentTodo(iapi.ctx, args[0], args[1])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete incident todo: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "todo %s deleted\n", args[1])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
