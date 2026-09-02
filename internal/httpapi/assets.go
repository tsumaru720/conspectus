package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"conspectus/internal/domain"
)

type assetIn struct {
	ClassID     int32  `json:"class_id"`
	Description string `json:"description"`
	Closed      *bool  `json:"closed"`
}

func (a *API) handleAssetsList(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	f := domain.AssetFilter{
		Query: r.URL.Query().Get("q"),
		Regex: r.URL.Query().Get("regex"),
		Sort:  r.URL.Query().Get("sort"),
	}
	if f.Query != "" && f.Regex != "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "q and regex are mutually exclusive")
		return
	}
	if v, ok := queryInt(r, "class_id"); ok {
		f.ClassID = v
	}
	if s := r.URL.Query().Get("closed"); s != "" {
		b := s == "true" || s == "1"
		f.Closed = &b
	}
	if s := r.URL.Query().Get("page"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			f.Page = n
		}
	}
	if s := r.URL.Query().Get("per_page"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			f.PerPage = n
		}
	}
	assets, total, err := repos.Assets().List(r.Context(), f)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	page, perPage := domain.Pagination(f.Page, f.PerPage)
	WriteData(w, http.StatusOK, assets, &Meta{Page: page, PerPage: perPage, Total: total})
}

func (a *API) handleAssetsCreate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	var in assetIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	asset, err := repos.Assets().Create(r.Context(), domain.Asset{
		ClassID:     in.ClassID,
		Description: strings.TrimSpace(in.Description),
	})
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusCreated, asset, nil)
}

func (a *API) handleAssetsGet(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	asset, err := repos.Assets().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, asset, nil)
}

func (a *API) handleAssetsUpdate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	var in assetIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	current, err := repos.Assets().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.ClassID != 0 {
		current.ClassID = in.ClassID
	}
	if in.Description != "" {
		current.Description = strings.TrimSpace(in.Description)
	}
	if in.Closed != nil {
		current.Closed = *in.Closed
	}
	asset, err := repos.Assets().Update(r.Context(), current)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, asset, nil)
}

func (a *API) handleAssetsDelete(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	force := r.URL.Query().Get("force") == "true"
	logs, payments, err := repos.Assets().Delete(r.Context(), id, force)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			body := map[string]any{
				"error": map[string]any{
					"code":        "conflict",
					"message":     err.Error(),
					"log_entries": logs,
					"payments":    payments,
				},
			}
			WriteJSON(w, http.StatusConflict, body)
			return
		}
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, map[string]any{
		"deleted":     true,
		"id":          id,
		"log_entries": logs,
		"payments":    payments,
	}, nil)
}

func (a *API) handleAssetsClose(w http.ResponseWriter, r *http.Request) { a.setAssetClosed(w, r, true) }
func (a *API) handleAssetsReopen(w http.ResponseWriter, r *http.Request) {
	a.setAssetClosed(w, r, false)
}

func (a *API) setAssetClosed(w http.ResponseWriter, r *http.Request, closed bool) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if err := repos.Assets().SetClosed(r.Context(), id, closed); err != nil {
		WriteDomainError(w, err)
		return
	}
	asset, err := repos.Assets().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, asset, nil)
}
