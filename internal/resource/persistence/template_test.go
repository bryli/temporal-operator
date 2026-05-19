// Licensed to Alexandre VILAIN under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Alexandre VILAIN licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package persistence

import (
	"strings"
	"testing"

	"github.com/alexandrevilain/temporal-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestTemplates(t *testing.T) {
	var s strings.Builder
	assert.NoError(t, templates[createDatabaseTemplate].Execute(&s, createDatabase{
		sqlBaseData: sqlBaseData{
			baseData: baseData{MTLSProvider: "linkerd"},
		},
	}))
	assert.Contains(t, s.String(), "curl -X POST http://localhost:4191/shutdown")
}

func TestTemplatesPasswordCommandPrefix(t *testing.T) {
	tests := map[string]struct {
		templateName string
		data         any
		wantContains string
		wantMissing  string
	}{
		"createDatabase with passwordCommand": {
			templateName: createDatabaseTemplate,
			data: createDatabase{
				sqlBaseData: sqlBaseData{
					PasswordCommandPrefix: `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$(/usr/bin/aws rds generate-db-auth-token --hostname mydb --port 5432 --username temporal)`,
				},
				Tool:           "temporal-sql-tool",
				ConnectionArgs: `--endpoint="mydb" --port="5432"`,
				DatabaseName:   "temporal",
			},
			wantContains: `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$(/usr/bin/aws rds generate-db-auth-token --hostname mydb --port 5432 --username temporal)`,
		},
		"createDatabase without passwordCommand": {
			templateName: createDatabaseTemplate,
			data: createDatabase{
				sqlBaseData:    sqlBaseData{},
				Tool:           "temporal-sql-tool",
				ConnectionArgs: `--endpoint="mydb" --port="5432"`,
				DatabaseName:   "temporal",
			},
			wantMissing: "export TEMPORAL_",
		},
		"createDatabaseV1_18 with passwordCommand": {
			templateName: createDatabaseTemplateV1_18,
			data: createDatabase{
				sqlBaseData: sqlBaseData{
					PasswordCommandPrefix: `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$(my-cmd)`,
				},
				Tool:           "temporal-sql-tool",
				ConnectionArgs: `--endpoint="mydb"`,
			},
			wantContains: `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$(my-cmd)`,
		},
		"setupSchema with passwordCommand": {
			templateName: setupSchemaTemplate,
			data: setupSchemaData{
				sqlBaseData: sqlBaseData{
					PasswordCommandPrefix: `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$(my-cmd --arg1)`,
				},
				Tool:           "temporal-sql-tool",
				ConnectionArgs: `--endpoint="mydb"`,
				InitialVersion: "0.0",
			},
			wantContains: `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$(my-cmd --arg1)`,
		},
		"updateSchema with passwordCommand": {
			templateName: updateSchemaTemplate,
			data: updateSchemaData{
				sqlBaseData: sqlBaseData{
					PasswordCommandPrefix: `export TEMPORAL_VISIBILITY_DATASTORE_PASSWORD=$(my-cmd)`,
				},
				Tool:           "temporal-sql-tool",
				ConnectionArgs: `--endpoint="mydb"`,
				SchemaDir:      "/etc/temporal/schema/postgresql/v12/temporal/versioned",
			},
			wantContains: `export TEMPORAL_VISIBILITY_DATASTORE_PASSWORD=$(my-cmd)`,
		},
		"updateSchema without passwordCommand": {
			templateName: updateSchemaTemplate,
			data: updateSchemaData{
				sqlBaseData:    sqlBaseData{},
				Tool:           "temporal-sql-tool",
				ConnectionArgs: `--endpoint="mydb"`,
				SchemaDir:      "/etc/temporal/schema/postgresql/v12/temporal/versioned",
			},
			wantMissing: "export TEMPORAL_",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var s strings.Builder
			err := templates[tc.templateName].Execute(&s, tc.data)
			assert.NoError(t, err)

			output := s.String()
			if tc.wantContains != "" {
				assert.Contains(t, output, tc.wantContains)
			}
			if tc.wantMissing != "" {
				assert.NotContains(t, output, tc.wantMissing)
			}
		})
	}
}

func TestPasswordCommandPrefix(t *testing.T) {
	builder := &SchemaScriptsConfigmapBuilder{}

	t.Run("returns empty when no SQL spec", func(t *testing.T) {
		spec := &v1beta1.DatastoreSpec{Name: "default"}
		result := builder.passwordCommandPrefix(spec)
		assert.Empty(t, result)
	})

	t.Run("returns empty when no passwordCommand", func(t *testing.T) {
		spec := &v1beta1.DatastoreSpec{
			Name: "default",
			SQL:  &v1beta1.SQLSpec{},
		}
		result := builder.passwordCommandPrefix(spec)
		assert.Empty(t, result)
	})

	t.Run("returns empty when passwordSecretRef takes precedence", func(t *testing.T) {
		spec := &v1beta1.DatastoreSpec{
			Name: "default",
			SQL: &v1beta1.SQLSpec{
				PasswordCommand: &v1beta1.PasswordCommandSpec{
					Command: "/usr/bin/aws",
					Args:    []string{"rds", "generate-db-auth-token"},
				},
			},
			PasswordSecretRef: &v1beta1.SecretKeyReference{
				Name: "my-secret",
			},
		}
		result := builder.passwordCommandPrefix(spec)
		assert.Empty(t, result)
	})

	t.Run("returns export line when only passwordCommand is set", func(t *testing.T) {
		spec := &v1beta1.DatastoreSpec{
			Name: "default",
			SQL: &v1beta1.SQLSpec{
				PasswordCommand: &v1beta1.PasswordCommandSpec{
					Command: "/usr/bin/aws",
					Args:    []string{"rds", "generate-db-auth-token", "--hostname", "mydb.rds.amazonaws.com"},
				},
			},
		}
		result := builder.passwordCommandPrefix(spec)
		assert.Equal(t, `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$('/usr/bin/aws' 'rds' 'generate-db-auth-token' '--hostname' 'mydb.rds.amazonaws.com')`, result)
	})

	t.Run("handles command with no args", func(t *testing.T) {
		spec := &v1beta1.DatastoreSpec{
			Name: "visibility",
			SQL: &v1beta1.SQLSpec{
				PasswordCommand: &v1beta1.PasswordCommandSpec{
					Command: "/usr/local/bin/get-token",
				},
			},
		}
		result := builder.passwordCommandPrefix(spec)
		assert.Equal(t, `export TEMPORAL_VISIBILITY_DATASTORE_PASSWORD=$('/usr/local/bin/get-token')`, result)
	})

	t.Run("handles args with spaces and special characters", func(t *testing.T) {
		spec := &v1beta1.DatastoreSpec{
			Name: "default",
			SQL: &v1beta1.SQLSpec{
				PasswordCommand: &v1beta1.PasswordCommandSpec{
					Command: "/bin/sh",
					Args:    []string{"-c", "echo -n test"},
				},
			},
		}
		result := builder.passwordCommandPrefix(spec)
		assert.Equal(t, `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$('/bin/sh' '-c' 'echo -n test')`, result)
	})

	t.Run("handles args with embedded single quotes", func(t *testing.T) {
		spec := &v1beta1.DatastoreSpec{
			Name: "default",
			SQL: &v1beta1.SQLSpec{
				PasswordCommand: &v1beta1.PasswordCommandSpec{
					Command: "/bin/sh",
					Args:    []string{"-c", "echo it's working"},
				},
			},
		}
		result := builder.passwordCommandPrefix(spec)
		assert.Equal(t, `export TEMPORAL_DEFAULT_DATASTORE_PASSWORD=$('/bin/sh' '-c' 'echo it'\''s working')`, result)
	})
}
