package models

func (ms *ModelSuite) createTrashable(title string) *Task {
	task := &Task{Title: title, Description: "Línea 1\nLínea 2", Color: "sage", Completed: true}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())
	ms.NoError(ms.DB.Reload(task))
	return task
}

func (ms *ModelSuite) countDeletedTasks() int {
	var n int
	ms.NoError(ms.DB.Store.Get(&n, "SELECT COUNT(*) FROM deleted_tasks"))
	return n
}

func (ms *ModelSuite) expireTrash() {
	ms.NoError(ms.DB.RawQuery("UPDATE deleted_tasks SET deleted_at = NOW() - INTERVAL '2 hours'").Exec())
}

func (ms *ModelSuite) Test_TrashTask_MovesRow() {
	task := ms.createTrashable("A la papelera")

	ms.NoError(TrashTask(ms.DB, task))

	count, err := ms.DB.Count(&Task{})
	ms.NoError(err)
	ms.Equal(0, count)
	ms.Equal(1, ms.countDeletedTasks())
}

func (ms *ModelSuite) Test_RestoreTask_PreservesFields() {
	task := ms.createTrashable("Vuelve igual")

	ms.NoError(TrashTask(ms.DB, task))
	restored, err := RestoreTask(ms.DB, task.ID)
	ms.NoError(err)

	ms.Equal(task.ID, restored.ID)
	ms.Equal(task.Title, restored.Title)
	ms.Equal(task.Description, restored.Description)
	ms.Equal(task.Color, restored.Color)
	ms.Equal(task.Completed, restored.Completed)
	ms.True(task.CreatedAt.Equal(restored.CreatedAt))
	ms.Equal(0, ms.countDeletedTasks())
}

func (ms *ModelSuite) Test_RestoreTask_Twice_NotInTrash() {
	task := ms.createTrashable("Dos veces")

	ms.NoError(TrashTask(ms.DB, task))
	_, err := RestoreTask(ms.DB, task.ID)
	ms.NoError(err)

	_, err = RestoreTask(ms.DB, task.ID)
	ms.ErrorIs(err, ErrNotInTrash)
}

func (ms *ModelSuite) Test_RestoreTask_Expired() {
	task := ms.createTrashable("Caducada")

	ms.NoError(TrashTask(ms.DB, task))
	ms.expireTrash()

	_, err := RestoreTask(ms.DB, task.ID)
	ms.ErrorIs(err, ErrNotInTrash)

	count, err := ms.DB.Count(&Task{})
	ms.NoError(err)
	ms.Equal(0, count)
}

func (ms *ModelSuite) Test_TrashTask_PurgesExpired() {
	old := ms.createTrashable("Antigua")
	ms.NoError(TrashTask(ms.DB, old))
	ms.expireTrash()

	recent := ms.createTrashable("Reciente")
	ms.NoError(TrashTask(ms.DB, recent))

	ms.Equal(1, ms.countDeletedTasks())
	var id string
	ms.NoError(ms.DB.Store.Get(&id, "SELECT id FROM deleted_tasks"))
	ms.Equal(recent.ID.String(), id)
}
