package httpapi

import (
	"net/http"

	"conspectus/internal/domain"
)

type paymentIn struct {
	AssetID int32         `json:"asset_id"`
	Date    string        `json:"date"`
	Amount  *domain.Money `json:"amount"`
}

func (a *API) handlePaymentsList(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	f, err := entryFilterFromQuery(r)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	payments, total, err := repos.Payments().List(r.Context(), f)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	page, perPage := domain.Pagination(f.Page, f.PerPage)
	WriteData(w, http.StatusOK, payments, &Meta{Page: page, PerPage: perPage, Total: total})
}

func (a *API) handlePaymentsCreate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	var in paymentIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.Amount == nil {
		WriteError(w, http.StatusBadRequest, "validation_error", "amount is required")
		return
	}
	if in.AssetID == 0 {
		WriteError(w, http.StatusBadRequest, "validation_error", "asset_id is required")
		return
	}
	if in.Date == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "date is required (YYYY-MM-DD)")
		return
	}
	at, err := a.parseDate(in.Date)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if err := a.rejectFuture(at); err != nil {
		WriteDomainError(w, err)
		return
	}
	created, err := repos.Payments().Create(r.Context(), domain.Payment{
		AssetID: in.AssetID, At: a.stampNow(at), Amount: *in.Amount,
	})
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusCreated, created, nil)
}

func (a *API) handlePaymentsGet(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	p, err := repos.Payments().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, p, nil)
}

func (a *API) handlePaymentsUpdate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	var in paymentIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	current, err := repos.Payments().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.AssetID != 0 {
		current.AssetID = in.AssetID
	}
	if in.Date != "" {
		at, err := a.parseDate(in.Date)
		if err != nil {
			WriteDomainError(w, err)
			return
		}
		if err := a.rejectFuture(at); err != nil {
			WriteDomainError(w, err)
			return
		}
		current.At = a.stampNow(at)
	}
	if in.Amount != nil {
		current.Amount = *in.Amount
	}
	updated, err := repos.Payments().Update(r.Context(), current)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, updated, nil)
}

func (a *API) handlePaymentsDelete(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if err := repos.Payments().Delete(r.Context(), id); err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, map[string]any{"deleted": true, "id": id}, nil)
}
