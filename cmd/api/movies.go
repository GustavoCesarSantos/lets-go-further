package main

import (
	"fmt"
	"net/http"
	"time"

	"greenlight.gustavosantos.net/internal/data"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "create a new movie")
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	movie := data.Movie{
		ID:        id,
		Title:     "Dummy title",
		Runtime:   102,
		Genres:    []string{"drama", "romance", "war"},
		Version:   1,
		CreatedAt: time.Now(),
	}
	writeJsonErr := app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, err)
	}
}
