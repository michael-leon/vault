package pki

import (
	"context"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func pathListKeys(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "keys/?$",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback:                    b.pathListKeysHandler,
				ForwardPerformanceStandby:   false,
				ForwardPerformanceSecondary: false,
			},
		},

		HelpSynopsis:    pathListKeysHelpSyn,
		HelpDescription: pathListKeysHelpDesc,
	}
}

const pathListKeysHelpSyn = ``
const pathListKeysHelpDesc = ``

func (b *backend) pathListKeysHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	var responseKeys []string
	responseInfo := make(map[string]interface{})

	entries, err := listKeys(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	for _, identifier := range entries {
		key, err := fetchKeyById(ctx, req.Storage, identifier)
		if err != nil {
			return nil, err
		}

		responseKeys = append(responseKeys, string(identifier))
		responseInfo[string(identifier)] = map[string]interface{}{
			"name": key.Name,
		}

	}
	return logical.ListResponseWithInfo(responseKeys, responseInfo), nil

}

func pathKey(b *backend) *framework.Path {
	pattern := "key/" + framework.GenericNameRegex("ref")
	return buildPathKey(b, pattern)
}

func buildPathKey(b *backend, pattern string) *framework.Path {
	return &framework.Path{
		Pattern: pattern,

		Fields: map[string]*framework.FieldSchema{
			"ref": {
				Type:        framework.TypeString,
				Description: `Reference to key; either "default" for the configured default key, an identifier of a key, or the name assigned to the key.`,
				Default:     "default",
			},
			"name": {
				Type:        framework.TypeString,
				Description: `Human-readable name for this key.`,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback:                    b.pathGetKeyHandler,
				ForwardPerformanceStandby:   false,
				ForwardPerformanceSecondary: false,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback:                    b.pathUpdateKeyHandler,
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback:                    b.pathDeleteKeyHandler,
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathKeysHelpSyn,
		HelpDescription: pathKeysHelpDesc,
	}
}

const pathKeysHelpSyn = ``
const pathKeysHelpDesc = ``

func (b *backend) pathGetKeyHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	keyRef := data.Get("ref").(string)
	if len(keyRef) == 0 {
		return logical.ErrorResponse("missing key reference"), nil
	}

	keyId, err := resolveKeyReference(ctx, req.Storage, keyRef)
	if err != nil {
		return nil, err
	}
	if keyId == "" {
		return logical.ErrorResponse("unable to resolve key id for reference" + keyRef), nil
	}

	key, err := fetchKeyById(ctx, req.Storage, keyId)
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"id":      key.ID,
			"name":    key.Name,
			"type":    key.PrivateKeyType,
			"backing": "", // This would show up as "Managed" in "type"
		},
	}, nil

}

func (b *backend) pathUpdateKeyHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	keyRef := data.Get("ref").(string)
	if len(keyRef) == 0 {
		return logical.ErrorResponse("missing key reference"), nil
	}

	keyId, err := resolveKeyReference(ctx, req.Storage, keyRef)
	if err != nil {
		return nil, err
	}
	if keyId == "" {
		return logical.ErrorResponse("unable to resolve key id for reference" + keyRef), nil
	}

	key, err := fetchKeyById(ctx, req.Storage, keyId)
	if err != nil {
		return nil, err
	}

	newName := data.Get("name").(string)
	if len(newName) > 0 && !nameMatcher.MatchString(newName) {
		return logical.ErrorResponse("new key name outside of valid character limits"), nil
	}

	if newName != key.Name {
		key.Name = newName

		err := writeKey(ctx, req.Storage, *key)

		if err != nil {
			return nil, err
		}
	}

	resp := &logical.Response{
		Data: map[string]interface{}{
			"id":      key.ID,
			"name":    key.Name,
			"type":    key.PrivateKeyType,
			"backing": "", // This would show up as "Managed" in "type"
		},
	}

	if len(newName) == 0 {
		resp.AddWarning("Name successfully deleted, you will now need to reference this key by it's Id: " + string(key.ID))
	}

	return resp, nil
}

func (b *backend) pathDeleteKeyHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	keyRef := data.Get("ref").(string)
	if len(keyRef) == 0 {
		return logical.ErrorResponse("missing key reference"), nil
	}

	keyId, err := resolveKeyReference(ctx, req.Storage, keyRef)
	if err != nil {
		return nil, err
	}
	if keyId == "" {
		return logical.ErrorResponse("unable to resolve key id for reference" + keyRef), nil
	}

	wasDefault, err := deleteKey(ctx, req.Storage, keyId)
	if err != nil {
		return nil, err
	}

	var response *logical.Response
	if wasDefault {
		response = &logical.Response{}
		response.AddWarning(fmt.Sprintf("Deleted key %v (via key_ref %v); this was configured as the default key. Operations without an explicit key will not work until a new default is configured.", string(keyId), keyRef))
	}

	return response, nil

}
