package taskmaster

import (
	// "errors"
	// "os"

	"github.com/glgaspar/front_desk/connection"
)

type Task struct {
	ID            string       `json:"id" db:"id"`
	Title         string       `json:"title" db:"title"`
	UserCreated   string       `json:"userCreated" db:"userCreated"`
	IdUserCreated int          `json:"idUserCreated" db:"idUserCreated"`
	Description   string       `json:"description" db:"description"`
	Assignee      string       `json:"assignee" db:"assignee"`
	Status        string       `json:"status" db:"status"`
	IdStatus      int          `json:"idStatus" db:"idStatus"`
	Tags          []tag        `json:"tags" db:"tags"`
	CreatedAt     string       `json:"createdAt" db:"createdAt"`
	UpdatedAt     string       `json:"updatedAt" db:"updatedAt"`
	Deadline      string       `json:"deadline" db:"deadline"`
	Comments      []comment    `json:"comments" db:"comments"`
	Attachments   []attachment `json:"attachments" db:"attachments"`
	Subtasks      []subtask    `json:"subtasks" db:"subtasks"`
	Workspace     string       `json:"workspace" db:"workspace"`
	IdWorkspace   int          `json:"idWorkspace" db:"idWorkspace"`
}

type tag struct {
	ID    string `json:"id" db:"id"`
	Name  string `json:"name" db:"name"`
	Color string `json:"color" db:"color"`
}

type comment struct {
	ID        string `json:"id" db:"id"`
	User      string `json:"user" db:"user"`
	Content   string `json:"content" db:"content"`
	CreatedAt string `json:"createdAt" db:"createdAt"`
}

type attachment struct {
	ID         string `json:"id" db:"id"`
	Filename   string `json:"filename" db:"filename"`
	FileType   string `json:"fileType" db:"fileType"`
	FileSize   int64  `json:"fileSize" db:"fileSize"`
	URL        string `json:"url" db:"url"`
	UploadedAt string `json:"uploadedAt" db:"uploadedAt"`
}
type subtask struct {
	ID            string `json:"id" db:"id"`
	Title         string `json:"title" db:"title"`
	UserCreated   string `json:"userCreated" db:"userCreated"`
	IdUserCreated int    `json:"idUserCreated" db:"idUserCreated"`
	Description   string `json:"description" db:"description"`
	Assignee      string `json:"assignee" db:"assignee"`
	Status        string `json:"status" db:"status"`
	IdStatus      int    `json:"idStatus" db:"idStatus"`
	CreatedAt     string `json:"createdAt" db:"createdAt"`
	UpdatedAt     string `json:"updatedAt" db:"updatedAt"`
	Deadline      string `json:"deadline" db:"deadline"`
	TaskID        string `json:"taskId" db:"taskId"`
}

func (t *Task) ListAll() (*[]Task, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// This function would typically interact with a database to retrieve all tasks.
	// For now, we return an empty slice.
	return &[]Task{}, nil
}

func (t *Task) GetTaskByID(id string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to retrieve a task by its ID.
	// For now, we return nil to indicate no task found.
	return nil
}

func (t *Task) CreateTask(task Task) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to create a new task.
	// For now, we return the task as if it was created successfully.
	return nil
}

func (t *Task) UpdateTask(id string, updatedTask Task) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to update an existing task.
	// For now, we return the updated task as if it was updated successfully.

	return nil
}

func (t *Task) DeleteTask(id string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to delete a task by its ID.
	// For now, we return true to indicate the task was "deleted" successfully.
	query := "UPDATE frontdesk.tasks set status = -1 WHERE id = ?"

	_, err = conn.Query(query, t.ID)
	if err != nil {
		return err
	}

	return nil
}

func (t *Task) AssignTaskToUser(taskID string, userID string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to assign a task to a user.
	// For now, we return a dummy task with the user assigned.
	
	return nil
}

func (t *Task) AddCommentToTask(taskID string, comment comment) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to add a comment to a task.
	// For now, we return the comment as if it was added successfully.
	t.Comments = append(t.Comments, comment) // Set the ID to the task ID for reference
	return nil
}

func (t *Task) AddAttachmentToTask(taskID string, attachment attachment) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to add an attachment to a task.
	// For now, we return the attachment as if it was added successfully.
	t.Attachments = append(t.Attachments, attachment) // Set the ID to the task ID for reference
	return nil
}

func (t *Task) AddSubtaskToTask(taskID string, subtask subtask) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to add a subtask to a task.
	// For now, we return the subtask as if it was added successfully.
	subtask.TaskID = taskID // Set the TaskID to associate it with the parent *task
	t.Subtasks = append(t.Subtasks, subtask) // Add the subtask to the task's subtasks
	return nil
}

func (t *Task) ListSubtasks(taskID string) (*[]subtask, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// This function would typically interact with a database to list all subtasks for a given task.
	// For now, we return an empty slice to indicate no subtasks found.
	return &[]subtask{}, nil
}

func (t *Task) UpdateSubtask(taskID string, subtaskID string, updatedSubtask subtask) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to update an existing subtask.
	// For now, we return the updated subtask as if it was updated successfully.
	updatedSubtask.ID = subtaskID  // Ensure the ID is set to the one being updated
	updatedSubtask.TaskID = taskID // Ensure the TaskID is set for reference
	return &updatedSubtask
}

func (t *Task) DeleteSubtask(taskID string, subtaskID string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to delete a subtask by its ID.
	// For now, we return true to indicate the subtask was "deleted" successfully.
	query := "UPDATE frontdesk.tasks set status = -1 WHERE id = ?"

	_, err = conn.Query(query, t.ID)
	if err != nil {
		return err
	}

	return nil
}
