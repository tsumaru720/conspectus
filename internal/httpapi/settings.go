package httpapi

import (
	"net/http"

	"conspectus/internal/domain"
	"conspectus/internal/settings"
)

// settingWriteIn is the body for creating and patching a single setting:
// any combination of its value, description and display flag.
type settingWriteIn struct {
	Value       *string `json:"value"`
	Description *string `json:"description"`
	Display     *bool   `json:"display"`
}

// handleSettingsGet returns every settings row in one payload: key, value,
// description and manage-page display flag. Secret-convention values are
// redacted.
func (a *API) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	rows, err := repos.Settings().List(r.Context())
	if err != nil {
		a.Log.Error("settings unavailable", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "storage unavailable")
		return
	}
	for i := range rows {
		if settings.IsSecretKey(rows[i].Key) {
			rows[i].Value = "(redacted)"
		}
	}
	WriteData(w, http.StatusOK, rows, nil)
}

// handleSettingCreate adds a settings row: the key comes from the path, the
// body seeds any combination of value, description and display. The key
// must be fresh - recreating an existing key is a conflict.
func (a *API) handleSettingCreate(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if reason, protected := protectedSettingKey(key); protected {
		WriteError(w, http.StatusBadRequest, "validation_error", reason, key)
		return
	}
	var in settingWriteIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	value, description, display := "", "", false
	if in.Value != nil {
		value = *in.Value
	}
	if in.Description != nil {
		description = *in.Description
	}
	if in.Display != nil {
		display = *in.Display
	}
	if err := a.Settings.Create(r.Context(), key, value, description, display); err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusCreated, redactedSetting(domain.Setting{
		Key: key, Value: value, Description: description, Display: display,
	}), nil)
}

// handleSettingPatch updates an existing setting's value, description
// and/or display; fields not sent are left alone, and absent keys are a
// 404 - POST creates rows.
func (a *API) handleSettingPatch(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if reason, protected := protectedSettingKey(key); protected {
		WriteError(w, http.StatusBadRequest, "validation_error", reason, key)
		return
	}
	var in settingWriteIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.Value == nil && in.Description == nil && in.Display == nil {
		WriteError(w, http.StatusBadRequest, "validation_error",
			"nothing to update: pass value and/or description and/or display")
		return
	}
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	if _, err := repos.Settings().Get(r.Context(), key); err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.Value != nil {
		if err := a.Settings.Set(r.Context(), key, *in.Value); err != nil {
			WriteDomainError(w, err)
			return
		}
	}
	if in.Description != nil {
		if err := a.Settings.SetDescription(r.Context(), key, *in.Description); err != nil {
			WriteDomainError(w, err)
			return
		}
	}
	if in.Display != nil {
		if err := a.Settings.SetDisplay(r.Context(), key, *in.Display); err != nil {
			WriteDomainError(w, err)
			return
		}
	}
	rows, err := repos.Settings().List(r.Context())
	if err != nil {
		a.Log.Error("settings unavailable", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "storage unavailable")
		return
	}
	for _, s := range rows {
		if s.Key != key {
			continue
		}
		WriteData(w, http.StatusOK, redactedSetting(s), nil)
		return
	}
	WriteData(w, http.StatusOK, map[string]string{"key": key}, nil)
}

// handleSettingKeyDelete removes a setting row outright - value,
// description and display flag all go, and subsequent reads fall back to
// the key's built-in default, or to nothing for keys without one.
// To just hide a key from the manage page, patch display:false instead.
func (a *API) handleSettingKeyDelete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if reason, protected := protectedSettingKey(key); protected {
		WriteError(w, http.StatusBadRequest, "validation_error", reason, key)
		return
	}
	if err := a.Settings.Delete(r.Context(), key); err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, map[string]string{"deleted": key}, nil)
}

// redactedSetting masks the value of secret-convention keys
// (*password*, *secret*, *token*, *basicauth*).
func redactedSetting(s domain.Setting) domain.Setting {
	if settings.IsSecretKey(s.Key) {
		s.Value = "(redacted)"
	}
	return s
}

// protectedSettingKey reports why a settings key may not be written through
// the API, and whether that applies. Both keys belong to the migration
// runner (db_migration_state is its crash-resume marker). read_only is not
// guarded: it is the CONSPECTUS_READ_ONLY environment variable, never a
// database setting, so a stored row is inert.
func protectedSettingKey(key string) (string, bool) {
	switch key {
	case "db_version", "db_migration_state":
		return "key is managed by the migration runner", true
	}
	return "", false
}
