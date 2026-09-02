package httpapi

import (
	"net/http"
	"strings"

	"conspectus/internal/domain"
)

type classIn struct {
	Description string `json:"description"`
}

func (a *API) handleClassesList(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	classes, err := repos.Classes().List(r.Context())
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, classes, nil)
}

func (a *API) handleClassesCreate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	var in classIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	class, err := repos.Classes().Create(r.Context(), domain.Class{Description: strings.TrimSpace(in.Description)})
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusCreated, class, nil)
}

func (a *API) handleClassesGet(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	class, err := repos.Classes().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, class, nil)
}

func (a *API) handleClassesUpdate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	var in classIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	current, err := repos.Classes().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.Description != "" {
		current.Description = strings.TrimSpace(in.Description)
	}
	class, err := repos.Classes().Update(r.Context(), current)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, class, nil)
}

func (a *API) handleClassesDelete(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if err := repos.Classes().Delete(r.Context(), id); err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, map[string]any{"deleted": true, "id": id}, nil)
}
