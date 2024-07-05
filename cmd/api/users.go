package main

import (
	"errors"
	"net/http"
	"time"

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
    permissionError := app.models.Permissions.AddForUser(user.ID, "movies:read")
    if permissionError != nil {
        app.serverErrorResponse(w, r, permissionError)
        return
    }
	token, tokenErr := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if tokenErr != nil {
		app.serverErrorResponse(w, r, tokenErr)
		return
	}
	app.background(func() {
		data := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}
		sendEmailErr := app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if sendEmailErr != nil {
			app.logger.Error(sendEmailErr.Error())
		}
	})
	writeJsonErr := app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, writeJsonErr)
	}
}

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlaintext string `json:"token"`
	}
	readJsonErr := app.readJSON(w, r, &input)
	if readJsonErr != nil {
		app.badRequestResponse(w, r, readJsonErr)
		return
	}
	v := validator.New()
	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	user, getUserErr := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if getUserErr != nil {
		switch {
		case errors.Is(getUserErr, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, getUserErr)
		}
		return
	}
	user.Activated = true
	updateUserErr := app.models.Users.Update(user)
	if updateUserErr != nil {
		switch {
		case errors.Is(updateUserErr, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, updateUserErr)
		}
		return
	}
	deleteTokenErr := app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if deleteTokenErr != nil {
		app.serverErrorResponse(w, r, deleteTokenErr)
		return
	}
	writeJsonErr := app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if writeJsonErr != nil {
		app.serverErrorResponse(w, r, writeJsonErr)
	}
}
