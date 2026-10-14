package actions

import (
	"net/url"
	"strings"

	"todo/models"

	"github.com/gofrs/uuid"
)

func (as *ActionSuite) createTask(title, description, color string) *models.Task {
	task := &models.Task{Title: title, Description: description, Color: color}
	verrs, err := as.DB.ValidateAndCreate(task)
	as.NoError(err)
	as.False(verrs.HasAny())
	as.NoError(as.DB.Reload(task))
	return task
}

func (as *ActionSuite) countDeletedTasks() int {
	var n int
	as.NoError(as.DB.Store.Get(&n, "SELECT COUNT(*) FROM deleted_tasks"))
	return n
}

func (as *ActionSuite) Test_TasksIndex_RendersCardGrid() {
	as.createTask("Con color", "Línea 1", "coral")

	res := as.HTML("/").Get()
	as.Equal(200, res.Code)

	body := res.Body.String()
	as.Contains(body, "data-card-grid")
	as.Contains(body, "data-task-card")
	as.Contains(body, "color-coral")
	as.Contains(body, "Línea 1")
	as.Contains(body, "Coral")
	as.Contains(body, "data-task-delete")
	as.NotContains(body, "data-confirm")
}

func (as *ActionSuite) Test_TasksIndex_EscapesDescription() {
	as.createTask("Con HTML", "<img src=x onerror=alert(1)>", "")

	res := as.HTML("/").Get()
	as.Equal(200, res.Code)
	as.Contains(res.Body.String(), "&lt;img")
	as.NotContains(res.Body.String(), "<img src=x")
}

func (as *ActionSuite) Test_TasksCreate_WithDescriptionAndColor() {
	res := as.HTML("/tasks").Post(url.Values{
		"title": {"Con detalles"}, "description": {"  Una descripción  "}, "color": {"sage"},
	})
	as.Equal(303, res.Code)

	task := &models.Task{}
	as.NoError(as.DB.First(task))
	as.Equal("Una descripción", task.Description)
	as.Equal("sage", task.Color)
}

func (as *ActionSuite) Test_TasksCreate_EmptyColor_Defaults() {
	res := as.HTML("/tasks").Post(url.Values{"title": {"Sin color"}, "color": {""}})
	as.Equal(303, res.Code)

	task := &models.Task{}
	as.NoError(as.DB.First(task))
	as.Equal(models.ColorDefault, task.Color)
}

func (as *ActionSuite) Test_TasksCreate_InvalidColor() {
	res := as.HTML("/tasks").Post(url.Values{"title": {"Color raro"}, "color": {"magenta"}})
	as.Equal(422, res.Code)
	as.Contains(res.Body.String(), "El color seleccionado no es válido.")

	count, err := as.DB.Count(&models.Task{})
	as.NoError(err)
	as.Equal(0, count)
}

func (as *ActionSuite) Test_TasksCreate_DescriptionTooLong() {
	res := as.HTML("/tasks").Post(url.Values{
		"title": {"Descripción larga"}, "description": {strings.Repeat("a", models.DescriptionMaxLength+1)},
	})
	as.Equal(422, res.Code)
	as.Contains(res.Body.String(), "La descripción no puede superar los 2000 caracteres.")

	count, err := as.DB.Count(&models.Task{})
	as.NoError(err)
	as.Equal(0, count)
}

func (as *ActionSuite) Test_TasksCreate_Fetch() {
	req := as.HTML("/tasks")
	req.Headers["X-Requested-With"] = "XMLHttpRequest"
	res := req.Post(url.Values{"title": {"Creada con fetch"}})
	as.Equal(201, res.Code)

	body := res.Body.String()
	as.Contains(body, "data-task-card")
	as.Contains(body, "Creada con fetch")
	as.NotContains(body, "<!DOCTYPE")
	as.NotContains(body, "<html")
}

func (as *ActionSuite) Test_TasksCreate_Fetch_Invalid() {
	req := as.HTML("/tasks")
	req.Headers["X-Requested-With"] = "XMLHttpRequest"
	res := req.Post(url.Values{"title": {""}})
	as.Equal(422, res.Code)

	body := res.Body.String()
	as.Contains(body, "data-composer")
	as.Contains(body, "El título es obligatorio.")
	as.NotContains(body, "<html")
}

func (as *ActionSuite) Test_TasksEdit_PrefillsDescriptionAndColor() {
	task := as.createTask("Para editar", "Descripción previa", "dusk")

	res := as.HTML("/tasks/%s/edit", task.ID).Get()
	as.Equal(200, res.Code)

	body := res.Body.String()
	as.Contains(body, ">Descripción previa</textarea>")
	as.Contains(body, `value="dusk" checked`)
}

func (as *ActionSuite) Test_TasksUpdate_PersistsDescriptionAndColor() {
	task := as.taskForEditing(true)

	res := as.HTML("/tasks/%s", task.ID).Put(url.Values{
		"title": {"Nuevo título"}, "description": {"  Nueva descripción  "}, "color": {"mint"},
	})
	as.Equal(303, res.Code)

	as.NoError(as.DB.Reload(task))
	as.Equal("Nuevo título", task.Title)
	as.Equal("Nueva descripción", task.Description)
	as.Equal("mint", task.Color)
	as.True(task.Completed)
}

func (as *ActionSuite) Test_TasksUpdate_InvalidColor() {
	task := as.taskForEditing(false)

	res := as.HTML("/tasks/%s", task.ID).Put(url.Values{"title": {"Otro título"}, "color": {"magenta"}})
	as.Equal(422, res.Code)
	as.Contains(res.Body.String(), "El color seleccionado no es válido.")

	as.NoError(as.DB.Reload(task))
	as.Equal("Título original", task.Title)
	as.Equal(models.ColorDefault, task.Color)
}

func (as *ActionSuite) Test_TasksUpdate_Fetch() {
	task := as.taskForEditing(false)

	req := as.HTML("/tasks/%s", task.ID)
	req.Headers["X-Requested-With"] = "XMLHttpRequest"
	res := req.Put(url.Values{"title": {"Título por fetch"}})
	as.Equal(200, res.Code)

	body := res.Body.String()
	as.Contains(body, "data-task-card")
	as.Contains(body, "Título por fetch")
	as.NotContains(body, "<html")
}

func (as *ActionSuite) Test_TasksUpdate_Fetch_Invalid() {
	task := as.taskForEditing(false)

	req := as.HTML("/tasks/%s", task.ID)
	req.Headers["X-Requested-With"] = "XMLHttpRequest"
	res := req.Put(url.Values{"title": {""}})
	as.Equal(422, res.Code)

	body := res.Body.String()
	as.Contains(body, "is-editing")
	as.Contains(body, "El título es obligatorio.")
	as.Contains(body, ">Título original</h3>")
	as.NotContains(body, "<html")
}

func (as *ActionSuite) Test_TasksToggle_Fetch() {
	task := as.taskForEditing(false)

	req := as.HTML("/tasks/%s/toggle", task.ID)
	req.Headers["X-Requested-With"] = "XMLHttpRequest"
	res := req.Put(url.Values{})
	as.Equal(200, res.Code)
	as.Contains(res.Body.String(), `aria-pressed="true"`)
	as.NotContains(res.Body.String(), "<html")

	as.NoError(as.DB.Reload(task))
	as.True(task.Completed)
}

func (as *ActionSuite) Test_TasksDestroy_MovesToTrash() {
	task := as.createTask("A la papelera", "", "")

	res := as.HTML("/tasks/%s", task.ID).Delete()
	as.Equal(303, res.Code)

	count, err := as.DB.Count(&models.Task{})
	as.NoError(err)
	as.Equal(0, count)
	as.Equal(1, as.countDeletedTasks())
}

func (as *ActionSuite) Test_TasksDestroy_Fetch() {
	task := as.createTask("Eliminada con fetch", "", "")

	req := as.HTML("/tasks/%s", task.ID)
	req.Headers["X-Requested-With"] = "XMLHttpRequest"
	res := req.Delete()
	as.Equal(204, res.Code)
	as.Empty(res.Body.String())
}

func (as *ActionSuite) Test_TasksRestore() {
	task := as.createTask("Se restaura", "Con descripción", "fog")

	res := as.HTML("/tasks/%s", task.ID).Delete()
	as.Equal(303, res.Code)

	res = as.HTML("/tasks/%s/restore", task.ID).Post(url.Values{})
	as.Equal(303, res.Code)
	as.Equal("/", res.Header().Get("Location"))

	restored := &models.Task{}
	as.NoError(as.DB.Find(restored, task.ID))
	as.Equal(task.ID, restored.ID)
	as.True(task.CreatedAt.Equal(restored.CreatedAt))
	as.Equal("Con descripción", restored.Description)
	as.Equal("fog", restored.Color)
	as.Equal(0, as.countDeletedTasks())
}

func (as *ActionSuite) Test_TasksRestore_Fetch() {
	task := as.createTask("Restaurada con fetch", "", "")
	as.Equal(303, as.HTML("/tasks/%s", task.ID).Delete().Code)

	req := as.HTML("/tasks/%s/restore", task.ID)
	req.Headers["X-Requested-With"] = "XMLHttpRequest"
	res := req.Post(url.Values{})
	as.Equal(200, res.Code)
	as.Contains(res.Body.String(), "data-task-card")
	as.Contains(res.Body.String(), "Restaurada con fetch")
}

func (as *ActionSuite) Test_TasksRestore_NotFound() {
	res := as.HTML("/tasks/%s/restore", uuid.Must(uuid.NewV4())).Post(url.Values{})
	as.Equal(404, res.Code)
}

func (as *ActionSuite) Test_TasksRestore_InvalidID() {
	res := as.HTML("/tasks/no-es-un-uuid/restore").Post(url.Values{})
	as.Equal(404, res.Code)
}

func (as *ActionSuite) Test_TasksRestore_Expired() {
	task := as.createTask("Caducada", "", "")
	as.Equal(303, as.HTML("/tasks/%s", task.ID).Delete().Code)
	as.NoError(as.DB.RawQuery("UPDATE deleted_tasks SET deleted_at = NOW() - INTERVAL '2 hours'").Exec())

	res := as.HTML("/tasks/%s/restore", task.ID).Post(url.Values{})
	as.Equal(404, res.Code)
}

func (as *ActionSuite) Test_TasksRestore_Twice() {
	task := as.createTask("Dos veces", "", "")
	as.Equal(303, as.HTML("/tasks/%s", task.ID).Delete().Code)

	as.Equal(303, as.HTML("/tasks/%s/restore", task.ID).Post(url.Values{}).Code)
	res := as.HTML("/tasks/%s/restore", task.ID).Post(url.Values{})
	as.Equal(404, res.Code)
}
