package actions

import (
	"todo/models"
	"todo/public"
	"todo/templates"

	"github.com/gobuffalo/buffalo/render"
)

// r renderiza páginas completas con el layout de la aplicación.
var r *render.Engine

// rFragment renderiza fragmentos HTML sin layout para las peticiones fetch.
// Hace falta un motor aparte porque Engine.HTML añade HTMLLayout cuando recibe
// un único nombre de plantilla.
var rFragment *render.Engine

// helpers son los helpers de plantilla compartidos por ambos motores.
var helpers = render.Helpers{
	// for non-bootstrap form helpers uncomment the lines
	// below and import "github.com/gobuffalo/helpers/forms"
	// forms.FormKey:     forms.Form,
	// forms.FormForKey:  forms.FormFor,
	"taskColors": func() []models.TaskColor { return models.TaskColors },
}

func init() {
	r = render.New(render.Options{
		// HTML layout to be used for all HTML requests:
		HTMLLayout: "application.plush.html",

		// fs.FS containing templates
		TemplatesFS: templates.FS(),

		// fs.FS containing assets
		AssetsFS: public.FS(),

		// Add template helpers here:
		Helpers: helpers,
	})

	rFragment = render.New(render.Options{
		TemplatesFS: templates.FS(),
		AssetsFS:    public.FS(),
		Helpers:     helpers,
	})
}
