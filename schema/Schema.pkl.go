// Code generated from Pkl module `jmgilman.pbl.schema`. DO NOT EDIT.
package schema

import (
	"context"

	"github.com/apple/pkl-go/pkl"
)

type Schema struct {
	Project *Project `pkl:"project"`
}

// LoadFromPath loads the pkl module at the given path and evaluates it into a Schema
func LoadFromPath(ctx context.Context, path string) (ret *Schema, err error) {
	evaluator, err := pkl.NewEvaluator(ctx, pkl.PreconfiguredOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		cerr := evaluator.Close()
		if err == nil {
			err = cerr
		}
	}()
	ret, err = Load(ctx, evaluator, pkl.FileSource(path))
	return ret, err
}

// Load loads the pkl module at the given source and evaluates it with the given evaluator into a Schema
func Load(ctx context.Context, evaluator pkl.Evaluator, source *pkl.ModuleSource) (*Schema, error) {
	var ret Schema
	if err := evaluator.EvaluateModule(ctx, source, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}
