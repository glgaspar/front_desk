package taskmaster

import (
	// "errors"
	// "os"

	"github.com/glgaspar/front_desk/connection"
)

type Workspace struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	CreatedAt   string `json:"createdAt" db:"createdAt"`
	UpdatedAt   string `json:"updatedAt" db:"updatedAt"`
}

func (w *Workspace) ListAllTasks() (*[]Task, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var taskList []Task
	var query string
	query = `
	SELECT 
		t.id, 
		t.title, 
		u.username as userCreated,
		t.idUserCreated,
		t.description,
		u.username as assignee,
		s.name as status,
		t.idStatus,
		-- t.tags,
		t.createdAt,
		t.updatedAt,
		t.deadline,
		w.workspace, 
		t.idWorkspace
	FROM taskmaster.tasks
	left join adm.users u on t.idUserCreated = u.id
	left join taskmaster.status s on t.idStatus = s.id
	left join taskmaster.workspaces w on t.idWorkspace = w.id
	WHERE status != -1`

	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.UserCreated,
			&task.IdUserCreated,
			&task.Description,
			&task.Assignee,
			&task.Status,
			&task.IdStatus,
			// &task.Tags, // Assuming Tags is a JSON field or similar, handle accordingly
			&task.CreatedAt,
			&task.UpdatedAt,
			&task.Deadline,
			&task.Workspace,
			&task.IdWorkspace,
		)
		if err != nil {
			return nil, err
		}
		taskList = append(taskList, task)
	}
	err = rows.Err()

	return &taskList, err
}

type Task struct {
	ID            string `json:"id" db:"id"`
	Title         string `json:"title" db:"title"`
	UserCreated   string `json:"userCreated" db:"userCreated"`
	IdUserCreated int    `json:"idUserCreated" db:"idUserCreated"`
	Description   string `json:"description" db:"description"`
	Assignee      string `json:"assignee" db:"assignee"`
	Status        string `json:"status" db:"status"`
	IdStatus      int    `json:"idStatus" db:"idStatus"`
	Tags          []tag  `json:"tags" db:"tags"`
	CreatedAt     string `json:"createdAt" db:"createdAt"`
	UpdatedAt     string `json:"updatedAt" db:"updatedAt"`
	Deadline      string `json:"deadline" db:"deadline"`
	Workspace     string `json:"workspace" db:"workspace"`
	IdWorkspace   int    `json:"idWorkspace" db:"idWorkspace"`
}

func (t *Task) GetTaskByID(id string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	var query string
	query = `
	SELECT 
		t.id, 
		t.title, 
		u.username as userCreated,
		t.idUserCreated,
		t.description,
		u.username as assignee,
		s.name as status,
		t.idStatus,
		-- t.tags,
		t.createdAt,
		t.updatedAt,
		t.deadline,
		w.workspace, 
		t.idWorkspace
	FROM taskmaster.tasks
	left join adm.users u on t.idUserCreated = u.id
	left join taskmaster.status s on t.idStatus = s.id
	left join taskmaster.workspaces w on t.idWorkspace = w.id
	WHERE t.id = $1`

	rows, err := conn.Query(query, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(
			&t.ID,
			&t.Title,
			&t.UserCreated,
			&t.IdUserCreated,
			&t.Description,
			&t.Assignee,
			&t.Status,
			&t.IdStatus,
			// &t.Tags, // Assuming Tags is a JSON field or similar, handle accordingly
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.Deadline,
			&t.Workspace,
			&t.IdWorkspace,
		)
		if err != nil {
			return err
		}
	}
	err = rows.Err()

	return nil
}

func (t *Task) CreateTask(workspaceId string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	INSERT INTO
	taskmaster.tasks (
		title,
		idUserCreated,
		description,
		assignee,
		idStatus,
		createdAt,
		updatedAt,
		deadline,
		idWorkspace
	)
	VALUES ($1, $2, $3, $4, $5, NOW(), NOW(), $6, $7)
	RETURNING id,
	title,
	userCreated,
	idUserCreated,
	description,
	assignee,
	status,
	idStatus,
	createdAt,
	updatedAt,
	deadline,
	workspace,
	idWorkspace`
	
	row, err := conn.Query(query, t.Title, t.IdUserCreated, t.Description, t.Assignee, t.IdStatus, t.Deadline, workspaceId)
	if err != nil {
		return err
	}

	for row.Next() {
		err := row.Scan(
			&t.ID,
			&t.Title,
			&t.UserCreated,
			&t.IdUserCreated,
			&t.Description,
			&t.Assignee,
			&t.Status,
			&t.IdStatus,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.Deadline,
			&t.Workspace,
			&t.IdWorkspace,
		)
		if err != nil {
			return err
		}
	}
	if row.Err() != nil {
		return row.Err()
	}
	return nil
}

func (t *Task) UpdateTask(id string, updatedTask Task) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

func (t *Task) DeleteTask(id string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := "UPDATE frontdesk.tasks set status = -1 WHERE id = $1"

	_, err = conn.Query(query, t.ID)
	if err != nil {
		return err
	}

	return nil
}

func (t *Task) AssignTaskToUser(userID string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	var query string
	query = `
	UPDATE taskmaster.tasks
	SET assignee = $1
	WHERE id = $2
	RETURNING select username as assignee from adm.users where id = $1`

	row, err := conn.Query(query, userID, t.ID)
	if err != nil {
		return err
	}

	for row.Next() {
		err := row.Scan(&t.Assignee)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *Task) ListSubtasks(taskID string) (*[]Subtask, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var subtasks []Subtask
	query := `
	SELECT
		s.id,
		s.title,
		s.userCreated,
		s.idUserCreated,
		s.description,
		s.assignee,
		s.status,
		s.idStatus,
		s.createdAt,
		s.updatedAt,
		s.deadline,
		s.taskId
	FROM taskmaster.subtasks s
	WHERE s.taskId = $1 AND s.status != -1`
	rows, err := conn.Query(query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var subtask Subtask
		err := rows.Scan(
			&subtask.ID,
			&subtask.Title,
			&subtask.UserCreated,
			&subtask.IdUserCreated,
			&subtask.Description,
			&subtask.Assignee,
			&subtask.Status,
			&subtask.IdStatus,
			&subtask.CreatedAt,
			&subtask.UpdatedAt,
			&subtask.Deadline,
			&subtask.TaskID,
		)
		if err != nil {
			return nil, err
		}
		subtasks = append(subtasks, subtask)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return &subtasks, nil
}

type tag struct {
	ID    string `json:"id" db:"id"`
	Name  string `json:"name" db:"name"`
	Color string `json:"color" db:"color"`
}

type Comment struct {
	ID        string `json:"id" db:"id"`
	User      string `json:"user" db:"user"`
	TaskID    string `json:"taskId" db:"taskId"`
	IDUser    int    `json:"idUser" db:"idUser"`
	Content   string `json:"content" db:"content"`
	CreatedAt string `json:"createdAt" db:"createdAt"`
}

func (c *Comment) AddCommentToTask(taskID string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	insert into taskmaster.comment (idTask,idUser,comment)
	values ($1, $2, $3)
	RETURNING *`
	newBill, err := conn.Query(query, taskID, c.IDUser, c.Content)
	if err != nil {
		return err
	}

	for newBill.Next() {
		newBill.Scan(&c.ID, &c.User, &c.TaskID, &c.IDUser, &c.Content, &c.CreatedAt)
	}

	return nil
}

type Attachment struct {
	ID         string `json:"id" db:"id"`
	Filename   string `json:"filename" db:"filename"`
	FileType   string `json:"fileType" db:"fileType"`
	FileSize   int64  `json:"fileSize" db:"fileSize"`
	URL        string `json:"url" db:"url"`
	UploadedAt string `json:"uploadedAt" db:"uploadedAt"`
}

func (a *Attachment) AddAttachmentToTask(taskID string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to add an attachment to a task.
	// For now, we return the attachment as if it was added successfully.
	// Set the ID to the task ID for reference
	return nil
}

type Subtask struct {
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

func (s *Subtask) AddSubtaskToTask(taskID string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

func (t *Subtask) UpdateSubtask() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to update an existing subtask.
	// For now, we return the updated subtask as if it was updated successfully.
	// updatedSubtask.ID = subtaskID  // Ensure the ID is set to the one being updated
	// updatedSubtask.TaskID = taskID // Ensure the TaskID is set for reference
	return nil
}

func (s *Subtask) DeleteSubtask() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	// This function would typically interact with a database to delete a subtask by its ID.
	// For now, we return true to indicate the subtask was "deleted" successfully.
	query := "UPDATE frontdesk.tasks set status = -1 WHERE id = ?"

	_, err = conn.Query(query, s.ID)
	if err != nil {
		return err
	}

	return nil
}
