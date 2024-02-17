package main

import (
	"errors"
	"net/http"

	"greenlight.gustavosantos.net/internal/data"
	"greenlight.gustavosantos.net/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	readJsonErr := app.readJSON(w, r, &input)
	if readJsonErr != nil {
		app.badRequestResponse(w, r, readJsonErr)
		return
	}
	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}
	err := user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	insertErr := app.models.Users.Insert(user)
	if insertErr != nil {
		switch {
		case errors.Is(insertErr, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, insertErr)
		}
		return
	}
    app.background(func() {
		sendEmailErr := app.mailer.Send(user.Email, "user_welcome.tmpl", user)
		if sendEmailErr != nil {
			app.logger.Error(sendEmailErr.Error())
		}
    })
	writeJsonErr := app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, writeJsonErr)
	}
}
