/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package verifiable

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	mockprovider "github.com/hyperledger/aries-framework-go/pkg/internal/mock/provider"
	mockstore "github.com/hyperledger/aries-framework-go/pkg/mock/storage"
	"github.com/hyperledger/aries-framework-go/pkg/store/verifiable"
)

const sampleCredentialName = "sampleVCName"
const sampleVCID = "http://example.edu/credentials/1989"

const vc = `
{ 
   "@context":[ 
      "https://www.w3.org/2018/credentials/v1"
   ],
   "id":"http://example.edu/credentials/1989",
   "type":"VerifiableCredential",
   "credentialSubject":{ 
      "id":"did:example:iuajk1f712ebc6f1c276e12ec21"
   },
   "issuer":{ 
      "id":"did:example:09s12ec712ebc6f1c671ebfeb1f",
      "name":"Example University"
   },
   "issuanceDate":"2020-01-01T10:54:01Z",
   "credentialStatus":{ 
      "id":"https://example.gov/status/65",
      "type":"CredentialStatusList2017"
   }
}
`

func TestNew(t *testing.T) {
	t.Run("test new command - success", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		handlers := cmd.GetHandlers()
		require.Equal(t, 7, len(handlers))
	})

	t.Run("test new command - vc store error", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{
				ErrOpenStoreHandle: fmt.Errorf("error opening the store"),
			},
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "new vc store")
		require.Nil(t, cmd)
	})
}

func TestValidateVC(t *testing.T) {
	t.Run("test register - success", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		vcReq := Credential{VerifiableCredential: vc}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer
		err = cmd.ValidateCredential(&b, bytes.NewBuffer(vcReqBytes))
		require.NoError(t, err)
	})

	t.Run("test register - invalid request", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		var b bytes.Buffer

		err = cmd.ValidateCredential(&b, bytes.NewBufferString("--"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "request decode")
	})

	t.Run("test register - validation error", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		vcReq := Credential{VerifiableCredential: ""}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer

		err = cmd.ValidateCredential(&b, bytes.NewBuffer(vcReqBytes))
		require.Error(t, err)
		require.Contains(t, err.Error(), "new credential")
	})
}

func TestSaveVC(t *testing.T) {
	t.Run("test save vc - success", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		vcReq := CredentialExt{
			Credential: Credential{VerifiableCredential: vc},
			Name:       sampleCredentialName,
		}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer
		err = cmd.SaveCredential(&b, bytes.NewBuffer(vcReqBytes))
		require.NoError(t, err)
	})

	t.Run("test save vc - invalid request", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		var b bytes.Buffer

		err = cmd.SaveCredential(&b, bytes.NewBufferString("--"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "request decode")
	})

	t.Run("test save vc - validation error", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		vcReq := CredentialExt{
			Credential: Credential{VerifiableCredential: ""},
			Name:       sampleCredentialName,
		}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer

		err = cmd.SaveCredential(&b, bytes.NewBuffer(vcReqBytes))
		require.Error(t, err)
		require.Contains(t, err.Error(), "new credential")
	})

	t.Run("test save vc - store error", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{
				Store: &mockstore.MockStore{
					ErrPut: fmt.Errorf("put error"),
				},
			},
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		vcReq := CredentialExt{
			Credential: Credential{VerifiableCredential: vc},
			Name:       sampleCredentialName,
		}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer

		err = cmd.SaveCredential(&b, bytes.NewBuffer(vcReqBytes))
		require.Error(t, err)
		require.Contains(t, err.Error(), "save vc")
	})
}

func TestGetVC(t *testing.T) {
	t.Run("test get vc - success", func(t *testing.T) {
		s := make(map[string][]byte)
		s["http://example.edu/credentials/1989"] = []byte(vc)

		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{Store: &mockstore.MockStore{Store: s}},
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		jsoStr := fmt.Sprintf(`{"id":"%s"}`, "http://example.edu/credentials/1989")

		var getRW bytes.Buffer
		cmdErr := cmd.GetCredential(&getRW, bytes.NewBufferString(jsoStr))
		require.NoError(t, cmdErr)

		response := Credential{}
		err = json.NewDecoder(&getRW).Decode(&response)
		require.NoError(t, err)

		// verify response
		require.NotEmpty(t, response)
		require.NotEmpty(t, response.VerifiableCredential)
	})

	t.Run("test get vc - invalid request", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		var b bytes.Buffer
		err = cmd.GetCredential(&b, bytes.NewBufferString("--"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "request decode")
	})

	t.Run("test get vc - no id in the request", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		jsoStr := fmt.Sprintf(`{}`)

		var b bytes.Buffer
		err = cmd.GetCredential(&b, bytes.NewBufferString(jsoStr))
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential id is mandatory")
	})

	t.Run("test get vc - store error", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{
				Store: &mockstore.MockStore{
					ErrGet: fmt.Errorf("get error"),
				},
			},
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		jsoStr := fmt.Sprintf(`{"id":"%s"}`, "http://example.edu/credentials/1989")

		var b bytes.Buffer
		err = cmd.GetCredential(&b, bytes.NewBufferString(jsoStr))
		require.Error(t, err)
		require.Contains(t, err.Error(), "get vc")
	})
}

func TestGetCredentialByName(t *testing.T) {
	t.Run("test get vc by name - success", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		// save vc with name
		vcReq := CredentialExt{
			Credential: Credential{VerifiableCredential: vc},
			Name:       sampleCredentialName,
		}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer
		err = cmd.SaveCredential(&b, bytes.NewBuffer(vcReqBytes))
		require.NoError(t, err)

		jsoStr := fmt.Sprintf(`{"name":"%s"}`, sampleCredentialName)

		var getRW bytes.Buffer
		cmdErr := cmd.GetCredentialByName(&getRW, bytes.NewBufferString(jsoStr))
		require.NoError(t, cmdErr)

		var response verifiable.CredentialRecord
		err = json.NewDecoder(&getRW).Decode(&response)
		require.NoError(t, err)

		// verify response
		require.NotEmpty(t, response)
		require.Equal(t, sampleCredentialName, response.Name)
		require.Equal(t, sampleVCID, response.ID)
	})

	t.Run("test get vc - invalid request", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		var b bytes.Buffer
		err = cmd.GetCredentialByName(&b, bytes.NewBufferString("--"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "request decode")
	})

	t.Run("test get vc - no name in the request", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		jsoStr := fmt.Sprintf(`{}`)

		var b bytes.Buffer
		err = cmd.GetCredentialByName(&b, bytes.NewBufferString(jsoStr))
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential name is mandatory")
	})

	t.Run("test get vc - store error", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{
				Store: &mockstore.MockStore{
					ErrGet: fmt.Errorf("get error"),
				},
			},
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		jsoStr := fmt.Sprintf(`{"name":"%s"}`, sampleCredentialName)

		var b bytes.Buffer
		err = cmd.GetCredentialByName(&b, bytes.NewBufferString(jsoStr))
		require.Error(t, err)
		require.Contains(t, err.Error(), "get vc by name")
	})
}

func TestGetCredentials(t *testing.T) {
	t.Run("test get credentials", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		// save vc with name
		vcReq := CredentialExt{
			Credential: Credential{VerifiableCredential: vc},
			Name:       sampleCredentialName,
		}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer
		err = cmd.SaveCredential(&b, bytes.NewBuffer(vcReqBytes))
		require.NoError(t, err)

		var getRW bytes.Buffer
		cmdErr := cmd.GetCredentials(&getRW, nil)
		require.NoError(t, cmdErr)

		var response CredentialRecordResult
		err = json.NewDecoder(&getRW).Decode(&response)
		require.NoError(t, err)

		// verify response
		require.NotEmpty(t, response)
		require.Equal(t, 1, len(response.Result))
	})
}

func TestGeneratePresentation(t *testing.T) {
	cmd, cmdErr := New(&mockprovider.Provider{
		StorageProviderValue: mockstore.NewMockStoreProvider(),
	})
	require.NotNil(t, cmd)
	require.NoError(t, cmdErr)

	t.Run("test generate presentation - success", func(t *testing.T) {
		vcReq := Credential{VerifiableCredential: vc}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer
		err = cmd.GeneratePresentation(&b, bytes.NewBuffer(vcReqBytes))
		require.NoError(t, err)

		// verify response
		var response Presentation
		err = json.NewDecoder(&b).Decode(&response)
		require.NoError(t, err)

		require.NotEmpty(t, response)
	})

	t.Run("test generate presentation - invalid request", func(t *testing.T) {
		var b bytes.Buffer

		err := cmd.GeneratePresentation(&b, bytes.NewBufferString("--"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "request decode")
	})

	t.Run("test generate presentation - validation error", func(t *testing.T) {
		vcReq := Credential{VerifiableCredential: ""}
		vcReqBytes, err := json.Marshal(vcReq)
		require.NoError(t, err)

		var b bytes.Buffer

		err = cmd.GeneratePresentation(&b, bytes.NewBuffer(vcReqBytes))
		require.Error(t, err)
		require.Contains(t, err.Error(), "generate vp - parse vc ")
	})
}

func TestGeneratePresentationByID(t *testing.T) {
	s := make(map[string][]byte)
	cmd, err := New(&mockprovider.Provider{
		StorageProviderValue: &mockstore.MockStoreProvider{Store: &mockstore.MockStore{Store: s}},
	})
	require.NotNil(t, cmd)
	require.NoError(t, err)

	t.Run("test generate presentation - success", func(t *testing.T) {
		s["http://example.edu/credentials/1989"] = []byte(vc)

		jsoStr := fmt.Sprintf(`{"id":"%s"}`, "http://example.edu/credentials/1989")

		var getRW bytes.Buffer
		cmdErr := cmd.GeneratePresentationByID(&getRW, bytes.NewBufferString(jsoStr))
		require.NoError(t, cmdErr)

		response := Presentation{}
		err = json.NewDecoder(&getRW).Decode(&response)
		require.NoError(t, err)

		// verify response
		require.NotEmpty(t, response)
		require.NotEmpty(t, response.VerifiablePresentation)
	})

	t.Run("test generate presentation - invalid request", func(t *testing.T) {
		var b bytes.Buffer
		err = cmd.GeneratePresentationByID(&b, bytes.NewBufferString("--"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "request decode")
	})

	t.Run("test generate presentation - no id in the request", func(t *testing.T) {
		jsoStr := fmt.Sprintf(`{}`)

		var b bytes.Buffer
		err = cmd.GeneratePresentationByID(&b, bytes.NewBufferString(jsoStr))
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential id is mandatory")
	})

	t.Run("test generate presentation - store error", func(t *testing.T) {
		cmd, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{
				Store: &mockstore.MockStore{
					ErrGet: fmt.Errorf("get error"),
				},
			},
		})
		require.NotNil(t, cmd)
		require.NoError(t, err)

		jsoStr := fmt.Sprintf(`{"id":"%s"}`, "http://example.edu/credentials/1989")

		var b bytes.Buffer
		err = cmd.GeneratePresentationByID(&b, bytes.NewBufferString(jsoStr))
		require.Error(t, err)
		require.Contains(t, err.Error(), "get vc")
	})
}
