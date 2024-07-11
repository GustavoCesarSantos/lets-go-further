package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"greenlight.gustavosantos.net/internal/data"
	"greenlight.gustavosantos.net/internal/validator"
)

func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title  string
		Genres []string
		data.Filters
	}
	v := validator.New()
	qs := r.URL.Query()
	input.Title = app.readString(qs, "title", "")
	input.Genres = app.readCSV(qs, "genres", []string{})
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "id")
	input.Filters.SortSafelist = []string{
		"id",
		"-id",
		"title",
		"-title",
		"year",
		"-year",
		"runtime",
		"-runtime",
	}
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	movies, metadata, getAllErr := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
	if getAllErr != nil {
		app.serverErrorResponse(w, r, getAllErr)
		return
	}
	writeJsonErr := app.writeJSON(w, http.StatusOK, envelope{"movies": movies, "metadata": metadata}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, writeJsonErr)
	}
}

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}
	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
	}
	insertErr := app.models.Movies.Insert(movie)
	if insertErr != nil {
		app.serverErrorResponse(w, r, insertErr)
		return
	}
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))
	writeJsonErr := app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, writeJsonErr)
	}
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	movie, getErr := app.models.Movies.Get(id)
	if getErr != nil {
		switch {
		case errors.Is(getErr, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, getErr)
		}
		return
	}
	writeJsonErr := app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	movie, getErr := app.models.Movies.Get(id)
	if getErr != nil {
		switch {
		case errors.Is(getErr, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, getErr)
		}
		return
	}
	if r.Header.Get("X-Expected-Version") != "" {
		if strconv.Itoa(int(movie.Version)) != r.Header.Get("X-Expected-Version") {
			app.editConflictResponse(w, r)
			return
		}
	}
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}
	readErr := app.readJSON(w, r, &input)
	if readErr != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	if input.Title != nil {
		movie.Title = *input.Title
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres
	}
	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	updateErr := app.models.Movies.Update(movie)
	if updateErr != nil {
		switch {
		case errors.Is(updateErr, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, updateErr)
		}
		return
	}
	writeJsonErr := app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	deleteErr := app.models.Movies.Delete(id)
	if deleteErr != nil {
		switch {
		case errors.Is(deleteErr, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, deleteErr)
		}
	}
	writeJsonErr := app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, err)
	}
}
