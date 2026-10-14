package models

import (
	"errors"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// TrashRetention es cuánto tiempo puede deshacerse una eliminación.
const TrashRetention = time.Hour

// ErrNotInTrash indica que la tarea no está en la papelera o que su ventana de
// retención ha caducado.
var ErrNotInTrash = errors.New("la tarea no está en la papelera o ha caducado")

func retentionSeconds() int {
	return int(TrashRetention.Seconds())
}

// TrashTask mueve la tarea a deleted_tasks y la elimina de tasks. De paso purga
// las entradas cuya ventana de retención ha caducado.
//
// La lista de columnas duplica la de Task: si tasks gana columnas, añádelas aquí
// y en RestoreTask.
func TrashTask(tx *pop.Connection, task *Task) error {
	err := tx.RawQuery(`INSERT INTO deleted_tasks (id, title, description, color, completed, created_at, updated_at, deleted_at)
		SELECT id, title, description, color, completed, created_at, updated_at, NOW() FROM tasks WHERE id = ?
		ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description,
			color = EXCLUDED.color, completed = EXCLUDED.completed, created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at, deleted_at = EXCLUDED.deleted_at`, task.ID).Exec()
	if err != nil {
		return err
	}

	if err := tx.Destroy(task); err != nil {
		return err
	}

	return tx.RawQuery(`DELETE FROM deleted_tasks WHERE deleted_at < NOW() - (? * INTERVAL '1 second')`, retentionSeconds()).Exec()
}

// RestoreTask devuelve a tasks una tarea eliminada dentro de la ventana de
// retención, conservando su id, sus datos y sus marcas de tiempo.
func RestoreTask(tx *pop.Connection, id uuid.UUID) (*Task, error) {
	n, err := tx.RawQuery(`INSERT INTO tasks (id, title, description, color, completed, created_at, updated_at)
		SELECT id, title, description, color, completed, created_at, updated_at FROM deleted_tasks
		WHERE id = ? AND deleted_at >= NOW() - (? * INTERVAL '1 second')
		ON CONFLICT (id) DO NOTHING`, id, retentionSeconds()).ExecWithCount()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotInTrash
	}

	if err := tx.RawQuery(`DELETE FROM deleted_tasks WHERE id = ?`, id).Exec(); err != nil {
		return nil, err
	}

	task := &Task{}
	if err := tx.Find(task, id); err != nil {
		return nil, err
	}
	return task, nil
}
