package server

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mockClient "k8s.io/client-go/kubernetes/fake"
)

const (
	testMutationConfName = "image-validation-admission"
	defaultCABundle      = "testByte"
	modCABundle          = "changedByte"
)

type certTestCase struct {
	name          string
	expectedError string
}

func TestCreateMutationConfig(t *testing.T) {

	tc := map[string]certTestCase{
		"test-1": {
			name:          testMutationConfName,
			expectedError: "",
		},
		"test-2": {
			name:          "wrongName",
			expectedError: "not found",
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			var testClient = mockClient.NewSimpleClientset()
			ctx := context.Background()
			testWebHook := &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: v1.ObjectMeta{
					Name: c.name,
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{{
					Name: "test-webhook",
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						CABundle: []byte(defaultCABundle),
					},
				}},
			}

			conf, err := testClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(ctx, testWebHook, v1.CreateOptions{})
			assert.NoError(t, err)
			updateName := conf.Name
			assert.Equal(t, c.name, updateName)

			err = createMutationConfig(ctx, []byte(modCABundle), testClient)

			if testMutationConfName == c.name {
				assert.NoError(t, err)
				conf, err := testClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, testMutationConfName, v1.GetOptions{})
				assert.NoError(t, err)
				caBundle := conf.Webhooks[0].ClientConfig.CABundle
				assert.Equal(t, []byte(modCABundle), caBundle)
			} else {
				log.Println(err)
				assert.Contains(t, err.Error(), c.expectedError)
			}
		})
	}
}
