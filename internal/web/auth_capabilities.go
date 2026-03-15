package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	integrationsAPIRefererURL           = appStoreBaseURL + "/access/integrations/api"
	integrationsIndividualKeysRefererURL = appStoreBaseURL + "/access/integrations/api/individual-keys"
)

var (
	ErrAPIKeyNotFound        = errors.New("api key not found")
	ErrAPIKeyRolesUnresolved = errors.New("api key roles could not be resolved")
)

func integrationsHeaders(referer string) http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/vnd.api+json, application/json, text/csv")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-CSRF-ITC", "[asc-ui]")
	headers.Set("Origin", appStoreBaseURL)
	headers.Set("Referer", referer)
	return headers
}

func olympusHeaders(referer string) http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Requested-With", "xsdr2$")
	if referer != "" {
		headers.Set("Referer", referer)
	}
	return headers
}

func (c *Client) doIrisV1Request(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.doRequestBase(ctx, irisV1BaseURL, method, path, body, integrationsHeaders(integrationsAPIRefererURL))
}

func (c *Client) doIrisV2Request(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.doRequestBase(ctx, irisV2BaseURL, method, path, body, integrationsHeaders(integrationsIndividualKeysRefererURL))
}

func (c *Client) doOlympusRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.doRequestBase(ctx, olympusBaseURL, method, path, body, olympusHeaders(integrationsIndividualKeysRefererURL))
}

type keyActor struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type teamAPIKey struct {
	KeyID       string
	Name        string
	Roles       []string
	Active      bool
	KeyType     string
	LastUsed    string
	GeneratedBy *keyActor
	RevokedBy   *keyActor
}

type APIKeyRoleLookup struct {
	KeyID       string    `json:"keyId"`
	Name        string    `json:"name,omitempty"`
	Kind        string    `json:"kind"`
	Roles       []string  `json:"roles"`
	RoleSource  string    `json:"roleSource"`
	Active      bool      `json:"active"`
	KeyType     string    `json:"keyType,omitempty"`
	LastUsed    string    `json:"lastUsed,omitempty"`
	Lookup      string    `json:"lookup"`
	GeneratedBy *keyActor `json:"generatedBy,omitempty"`
	RevokedBy   *keyActor `json:"revokedBy,omitempty"`
}

func fullName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func (c *Client) listTeamKeys(ctx context.Context) ([]teamAPIKey, error) {
	body, err := c.doIrisV1Request(ctx, http.MethodGet, "/apiKeys?include=createdBy,revokedBy,provider&sort=-isActive,-revokingDate&limit=2000", nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				LastUsed     string   `json:"lastUsed"`
				Roles        []string `json:"roles"`
				Nickname     string   `json:"nickname"`
				RevokingDate string   `json:"revokingDate"`
				AllAppsVisible bool   `json:"allAppsVisible"`
				CanDownload  bool     `json:"canDownload"`
				IsActive     bool     `json:"isActive"`
				KeyType      string   `json:"keyType"`
			} `json:"attributes"`
			Relationships struct {
				CreatedBy struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"createdBy"`
				RevokedBy struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"revokedBy"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			} `json:"attributes"`
		} `json:"included"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse team keys response: %w", err)
	}

	users := make(map[string]string, len(payload.Included))
	for _, item := range payload.Included {
		if item.Type != "users" {
			continue
		}
		users[item.ID] = fullName(item.Attributes.FirstName, item.Attributes.LastName)
	}

	keys := make([]teamAPIKey, 0, len(payload.Data))
	for _, item := range payload.Data {
		key := teamAPIKey{
			KeyID:    strings.TrimSpace(item.ID),
			Name:     strings.TrimSpace(item.Attributes.Nickname),
			Roles:    append([]string(nil), item.Attributes.Roles...),
			Active:   item.Attributes.IsActive,
			KeyType:  strings.TrimSpace(item.Attributes.KeyType),
			LastUsed: strings.TrimSpace(item.Attributes.LastUsed),
		}
		if item.Relationships.CreatedBy.Data != nil {
			id := strings.TrimSpace(item.Relationships.CreatedBy.Data.ID)
			key.GeneratedBy = &keyActor{ID: id, Name: strings.TrimSpace(users[id])}
		}
		if item.Relationships.RevokedBy.Data != nil {
			id := strings.TrimSpace(item.Relationships.RevokedBy.Data.ID)
			key.RevokedBy = &keyActor{ID: id, Name: strings.TrimSpace(users[id])}
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (c *Client) LookupAPIKeyRoles(ctx context.Context, keyID string) (*APIKeyRoleLookup, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}

	keys, err := c.listTeamKeys(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range keys {
		if item.KeyID != keyID {
			continue
		}
		return &APIKeyRoleLookup{
			KeyID:       item.KeyID,
			Name:        item.Name,
			Kind:        "team",
			Roles:       append([]string(nil), item.Roles...),
			RoleSource:  "key",
			Active:      item.Active,
			KeyType:     item.KeyType,
			LastUsed:    item.LastUsed,
			Lookup:      "team_keys",
			GeneratedBy: item.GeneratedBy,
			RevokedBy:   item.RevokedBy,
		}, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrAPIKeyNotFound, keyID)
}
