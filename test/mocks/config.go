package mocks

import (
	"fmt"
	"path/filepath"

	"golang.org/x/oauth2"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/mevansam/goforms/forms"

	cb_mocks "github.com/appbricks/cloud-builder/test/mocks"
)

func NewMockConfig(sourceDirPath string) (config.Config, error) {

	var (
		err error
		
		testRecipePath string

		tgt       *target.Target
		inputForm forms.InputForm
	)

	// set up a target context instance to use for tests
	if testRecipePath, err = filepath.Abs(fmt.Sprintf("%s/../../cloud-builder/test/fixtures/recipes", sourceDirPath)); err != nil {
		return nil, err
	}

	tgtCtx := cb_mocks.NewTargetMockContext(testRecipePath)

	if tgt, err = tgtCtx.NewTarget("test:basic", "aws"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Recipe.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_input_1", "aa"); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_input_2", "cookbook"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Provider.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("region", "us-east-1"); err != nil {
		return nil, err
	}
	tgt.NodeID = "1d812616-5955-4bc6-8b67-ec3f0f12a756"
	tgt.RSAPublicKey = "PubKey1"
	tgtCtx.SaveTarget("", tgt)

	if tgt, err = tgtCtx.NewTarget("test:basic", "aws"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Recipe.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_input_1", "bb"); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_input_2", "cookbook"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Provider.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("region", "us-east-1"); err != nil {
		return nil, err
	}
	tgt.NodeID = "1d2a49d7-330b-4beb-a102-33049869e472"
	tgt.RSAPublicKey = "PubKey2"
	tgtCtx.SaveTarget("", tgt)

	if tgt, err = tgtCtx.NewTarget("test:basic", "aws"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Recipe.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_input_1", "cc"); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_input_2", "cookbook"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Provider.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("region", "us-east-1"); err != nil {
		return nil, err
	}
	tgt.RSAPublicKey = "PubKey3"
	tgtCtx.SaveTarget("", tgt)

	if tgt, err = tgtCtx.NewTarget("test:simple", "aws"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Recipe.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("name", "test-simple-deployment"); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_simple_input_1", "testsimple1"); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("test_simple_input_2", "testsimple2"); err != nil {
		return nil, err
	}
	if inputForm, err = tgt.Provider.InputForm(); err != nil {
		return nil, err
	}
	if err = inputForm.SetFieldValue("region", "us-west-2"); err != nil {
		return nil, err
	}
	tgt.NodeID = "126e0de1-d422-4200-9486-25b108d6cc8d"	
	tgt.RSAPublicKey = "PubKey4"
	tgtCtx.SaveTarget("", tgt)

	// set up an auth context instance to use for tests
	authContext := config.NewAuthContext()
	authContext.SetToken(
		(&oauth2.Token{}).WithExtra(
			map[string]interface{}{
				"id_token": "mock authorization token",
			},
		),
	)
	return cb_mocks.NewMockConfig(authContext, nil, tgtCtx), nil
}