package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// TitleMaxLength es el número máximo de caracteres permitido en el título.
const TitleMaxLength = 200

// DescriptionMaxLength es el número máximo de caracteres permitido en la descripción.
const DescriptionMaxLength = 2000

// ColorDefault es la clave del color que se usa cuando no se elige ninguno.
const ColorDefault = "default"

// TaskColor es un color de la paleta cerrada de tarjetas.
// Hex es documental: el color real se aplica en CSS con la clase "color-<Key>".
type TaskColor struct {
	Key   string
	Label string
	Hex   string
}

// TaskColors es la paleta cerrada, en orden de presentación.
var TaskColors = []TaskColor{
	{"default", "Predeterminado", "#ffffff"},
	{"coral", "Coral", "#faafa8"},
	{"peach", "Melocotón", "#f39f76"},
	{"sand", "Arena", "#fff8b8"},
	{"mint", "Menta", "#e2f6d3"},
	{"sage", "Salvia", "#b4ddd3"},
	{"fog", "Niebla", "#d4e4ed"},
	{"storm", "Tormenta", "#aeccdc"},
	{"dusk", "Anochecer", "#d3bfdb"},
	{"blossom", "Flor", "#f6e2dd"},
	{"clay", "Arcilla", "#e9e3d4"},
	{"chalk", "Tiza", "#efeff1"},
}

// IsValidColor indica si la clave pertenece a la paleta.
func IsValidColor(key string) bool {
	for _, c := range TaskColors {
		if c.Key == key {
			return true
		}
	}
	return false
}

// Task representa una tarea de la lista.
//
// Si se añaden columnas a la tabla tasks, actualiza también las consultas
// INSERT ... SELECT de models/trash.go: si no, Deshacer las perderá.
type Task struct {
	ID          uuid.UUID `json:"id" db:"id" form:"-"`
	Title       string    `json:"title" db:"title" form:"title"`
	Description string    `json:"description" db:"description" form:"description"`
	Color       string    `json:"color" db:"color" form:"color"`
	Completed   bool      `json:"completed" db:"completed" form:"-"`
	CreatedAt   time.Time `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at" form:"-"`
}

// Tasks es una colección de Task.
type Tasks []Task

// BeforeValidate normaliza un color vacío al color por defecto.
func (t *Task) BeforeValidate(tx *pop.Connection) error {
	t.Color = strings.TrimSpace(t.Color)
	if t.Color == "" {
		t.Color = ColorDefault
	}
	return nil
}

// Validate se ejecuta en cada llamada a los métodos "pop.Validate*".
func (t *Task) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()

	title := strings.TrimSpace(t.Title)
	if title == "" {
		verrs.Add("title", "El título es obligatorio.")
	} else if len([]rune(title)) > TitleMaxLength {
		verrs.Add("title", fmt.Sprintf("El título no puede superar los %d caracteres.", TitleMaxLength))
	}

	if len([]rune(t.Description)) > DescriptionMaxLength {
		verrs.Add("description", fmt.Sprintf("La descripción no puede superar los %d caracteres.", DescriptionMaxLength))
	}

	if !IsValidColor(t.Color) {
		verrs.Add("color", "El color seleccionado no es válido.")
	}

	return verrs, nil
}
