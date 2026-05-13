package echo

import (
	"encoding/json"

	"github.com/cryptowizard0/encrypttags/echo/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

type Echo struct {
	spawnSecret string
}

func New(env vmmSchema.Env) (*Echo, error) {
	return &Echo{
		spawnSecret: encryptedParam(env.Meta.Params, "SpawnSecret"),
	}, nil
}

func Spawn(env vmmSchema.Env) (vmmSchema.Vm, error) {
	return New(env)
}

func (e *Echo) Apply(_ string, meta vmmSchema.Meta) vmmSchema.Result {
	return vmmSchema.Result{
		Output: map[string]string{
			"SpawnSecret": e.spawnSecret,
			"Secret":      encryptedParam(meta.Params, "Secret"),
			"Plain":       meta.Params["Plain"],
		},
	}
}

func (e *Echo) Checkpoint() (string, error) {
	payload := map[string]string{
		"Module-Format": schema.ModuleFormat,
	}
	by, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(by), nil
}

func (e *Echo) Restore(data string) error {
	var payload map[string]string
	return json.Unmarshal([]byte(data), &payload)
}

func (e *Echo) Close() error {
	return nil
}

func encryptedParam(params map[string]string, name string) string {
	return params[name]
}
