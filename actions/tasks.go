package actions

import (
	"errors"
	"net/http"
	"strings"

	"todo/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// isFetch indica si la petición viene del JavaScript de la página, que espera
// un fragmento HTML en lugar de una redirección.
func isFetch(c buffalo.Context) bool {
	return c.Request().Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// setCardState prepara el contexto del partial tasks/_card: card es lo que
// muestra la vista y form lo que rellena el formulario de edición.
func setCardState(c buffalo.Context, card, form *models.Task, editing bool, verrs *validate.Errors) {
	c.Set("card", card)
	c.Set("cardForm", form)
	c.Set("cardEditing", editing)
	c.Set("cardErrors", verrs)
}

// setListDefaults define las variables que necesita cada tarjeta de la lista.
// Plush falla con "unknown identifier" si alguna no existe.
func setListDefaults(c buffalo.Context) {
	c.Set("cardEditing", false)
	c.Set("cardErrors", validate.NewErrors())
}

// renderCard responde con el fragmento de una tarjeta, sin layout.
func renderCard(c buffalo.Context, status int) error {
	return c.Render(status, rFragment.HTML("tasks/_card.plush.html"))
}

// TasksIndex muestra la lista de tareas y el formulario de creación.
func TasksIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	tasks := models.Tasks{}
	if err := tx.Order("created_at asc, id asc").All(&tasks); err != nil {
		return err
	}

	setListDefaults(c)
	c.Set("tasks", tasks)
	c.Set("task", &models.Task{Color: models.ColorDefault})
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/index.plush.html"))
}

// TasksCreate crea una tarea nueva a partir del formulario.
func TasksCreate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := c.Bind(task); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(task.Title)
	task.Description = strings.TrimSpace(task.Description)

	verrs, err := tx.ValidateAndCreate(task)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		c.Set("task", task)
		c.Set("errors", verrs)
		if isFetch(c) {
			return c.Render(http.StatusUnprocessableEntity, rFragment.HTML("tasks/_composer.plush.html"))
		}

		tasks := models.Tasks{}
		if err := tx.Order("created_at asc, id asc").All(&tasks); err != nil {
			return err
		}

		setListDefaults(c)
		c.Set("tasks", tasks)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/index.plush.html"))
	}

	if isFetch(c) {
		setCardState(c, task, task, false, validate.NewErrors())
		return renderCard(c, http.StatusCreated)
	}

	c.Flash().Add("success", "Tarea creada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksEdit muestra el formulario de edición de una tarea existente.
func TasksEdit(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("task", task)
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/edit.plush.html"))
}

// TasksUpdate reemplaza el título, la descripción y el color, conservando los
// demás datos de la tarea.
func TasksUpdate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	original := *task

	input := struct {
		Title       string `form:"title" json:"title"`
		Description string `form:"description" json:"description"`
		Color       string `form:"color" json:"color"`
	}{}
	if err := c.Bind(&input); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(input.Title)
	task.Description = strings.TrimSpace(input.Description)
	task.Color = input.Color

	verrs, err := tx.ValidateAndUpdate(task)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		if isFetch(c) {
			setCardState(c, &original, task, true, verrs)
			return renderCard(c, http.StatusUnprocessableEntity)
		}
		c.Set("task", task)
		c.Set("errors", verrs)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/edit.plush.html"))
	}

	if isFetch(c) {
		setCardState(c, task, task, false, validate.NewErrors())
		return renderCard(c, http.StatusOK)
	}

	c.Flash().Add("success", "Tarea actualizada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksToggle alterna una tarea entre completada y pendiente.
func TasksToggle(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	task.Completed = !task.Completed
	if err := tx.Update(task); err != nil {
		return err
	}

	if isFetch(c) {
		setCardState(c, task, task, false, validate.NewErrors())
		return renderCard(c, http.StatusOK)
	}

	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksDestroy elimina una tarea. La guarda en la papelera técnica para que la
// interfaz pueda deshacer la eliminación durante models.TrashRetention.
func TasksDestroy(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := models.TrashTask(tx, task); err != nil {
		return err
	}

	if isFetch(c) {
		return c.Render(http.StatusNoContent, nil)
	}

	c.Flash().Add("success", "Tarea eliminada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksRestore deshace la eliminación de una tarea que sigue en la papelera.
func TasksRestore(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	id, err := uuid.FromString(c.Param("task_id"))
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	task, err := models.RestoreTask(tx, id)
	if errors.Is(err, models.ErrNotInTrash) {
		return c.Error(http.StatusNotFound, err)
	}
	if err != nil {
		return err
	}

	if isFetch(c) {
		setCardState(c, task, task, false, validate.NewErrors())
		return renderCard(c, http.StatusOK)
	}

	c.Flash().Add("success", "Tarea restaurada.")
	return c.Redirect(http.StatusSeeOther, "/")
}
