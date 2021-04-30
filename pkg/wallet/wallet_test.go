package wallet

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	walletTemplate = "testWallet"
)

func TestNewWallet(t *testing.T) {
	wallet := checkWalletConstructor(t)
	require.NotNil(t, wallet)
}

func TestNewWalletFromFile_Negative_EmptyFile(t *testing.T) {
	_ = checkWalletConstructor(t)
	walletFromFile, err2 := NewWalletFromFile(walletTemplate)
	require.Errorf(t, err2, "EOF")
	require.Nil(t, walletFromFile)
}

func TestNewWalletFromFile_Negative_NoFile(t *testing.T) {
	_, err := NewWalletFromFile(walletTemplate)
	require.Errorf(t, err, "open testWallet: no such file or directory")
}

func TestCreateAccount(t *testing.T) {
	wallet := checkWalletConstructor(t)

	errAcc := wallet.CreateAccount("testName", "testPass")
	require.NoError(t, errAcc)
	accounts := wallet.Accounts
	require.Len(t, accounts, 1)
}

func TestAddAccount(t *testing.T) {
	wallet := checkWalletConstructor(t)

	wallet.AddAccount(&Account{
		privateKey:   nil,
		publicKey:    nil,
		wif:          "",
		Address:      "real",
		EncryptedWIF: "",
		Label:        "",
		Contract:     nil,
		Locked:       false,
		Default:      false,
	})
	accounts := wallet.Accounts
	require.Len(t, accounts, 1)

	require.Error(t, wallet.RemoveAccount("abc"))
	require.Len(t, wallet.Accounts, 1)
	require.NoError(t, wallet.RemoveAccount("real"))
	require.Len(t, wallet.Accounts, 0)
}

func TestPath(t *testing.T) {
	wallet := checkWalletConstructor(t)

	path := wallet.Path()
	require.NotEmpty(t, path)
}

func TestSave(t *testing.T) {
	file, err := ioutil.TempFile("", walletTemplate)
	require.NoError(t, err)
	wallet, err := NewWallet(file.Name())
	require.NoError(t, err)
	wallet.AddAccount(&Account{
		privateKey:   nil,
		publicKey:    nil,
		wif:          "",
		Address:      "",
		EncryptedWIF: "",
		Label:        "",
		Contract:     nil,
		Locked:       false,
		Default:      false,
	})

	t.Cleanup(func() {
		removeWallet(t, file.Name())
	})
	errForSave := wallet.Save()
	require.NoError(t, errForSave)

	openedWallet, err := NewWalletFromFile(wallet.path)
	require.NoError(t, err)
	require.Equal(t, wallet.Accounts, openedWallet.Accounts)

	t.Run("change and rewrite", func(t *testing.T) {
		err := openedWallet.CreateAccount("test", "pass")
		require.NoError(t, err)

		w2, err := NewWalletFromFile(openedWallet.path)
		require.NoError(t, err)
		require.Equal(t, 2, len(w2.Accounts))
		require.NoError(t, w2.Accounts[1].Decrypt("pass"))
		require.Equal(t, openedWallet.Accounts, w2.Accounts)
	})
}

func TestJSONMarshallUnmarshal(t *testing.T) {
	wallet := checkWalletConstructor(t)

	bytes, err := wallet.JSON()
	require.NoError(t, err)
	require.NotNil(t, bytes)

	unmarshalledWallet := &Wallet{}
	errUnmarshal := json.Unmarshal(bytes, unmarshalledWallet)

	require.NoError(t, errUnmarshal)
	require.Equal(t, wallet.Version, unmarshalledWallet.Version)
	require.Equal(t, wallet.Accounts, unmarshalledWallet.Accounts)
	require.Equal(t, wallet.Scrypt, unmarshalledWallet.Scrypt)
}

func checkWalletConstructor(t *testing.T) *Wallet {
	file, err := ioutil.TempFile("", walletTemplate)
	require.NoError(t, err)
	wallet, err := NewWallet(file.Name())
	t.Cleanup(func() {
		removeWallet(t, file.Name())
	})
	require.NoError(t, err)
	return wallet
}

func removeWallet(t *testing.T, walletPath string) {
	err := os.RemoveAll(walletPath)
	require.NoError(t, err)
}

func TestWallet_AddToken(t *testing.T) {
	w := checkWalletConstructor(t)
	tok := NewToken(util.Uint160{1, 2, 3}, "Rubl", "RUB", 2, manifest.NEP17StandardName)
	require.Equal(t, 0, len(w.Extra.Tokens))
	w.AddToken(tok)
	require.Equal(t, 1, len(w.Extra.Tokens))
	require.Error(t, w.RemoveToken(util.Uint160{4, 5, 6}))
	require.Equal(t, 1, len(w.Extra.Tokens))
	require.NoError(t, w.RemoveToken(tok.Hash))
	require.Equal(t, 0, len(w.Extra.Tokens))
}

func TestWallet_GetAccount(t *testing.T) {
	wallet := checkWalletConstructor(t)
	accounts := []*Account{
		{
			Contract: &Contract{
				Script: []byte{0, 1, 2, 3},
			},
		},
		{
			Contract: &Contract{
				Script: []byte{3, 2, 1, 0},
			},
		},
	}

	for _, acc := range accounts {
		acc.Address = address.Uint160ToString(acc.Contract.ScriptHash())
		wallet.AddAccount(acc)
	}

	for i, acc := range accounts {
		h := acc.Contract.ScriptHash()
		assert.Equal(t, acc, wallet.GetAccount(h), "can't get %d account", i)
	}
}

func TestWalletGetChangeAddress(t *testing.T) {
	w1, err := NewWalletFromFile("testdata/wallet1.json")
	require.NoError(t, err)
	sh := w1.GetChangeAddress()
	// No default address, the first one is used.
	expected, err := address.StringToUint160("NTh9TnZTstvAePEYWDGLLxidBikJE24uTo")
	require.NoError(t, err)
	require.Equal(t, expected, sh)
	w2, err := NewWalletFromFile("testdata/wallet2.json")
	require.NoError(t, err)
	sh = w2.GetChangeAddress()
	// Default address.
	expected, err = address.StringToUint160("NUREbqw2kfbPgDeEz8Dac2QxntGGqqFMm7")
	require.NoError(t, err)
	require.Equal(t, expected, sh)
}

func TestWalletForExamples(t *testing.T) {
	const (
		examplesDir  = "../../examples"
		walletFile   = "my_wallet.json"
		walletPass   = "qwerty"
		accountLabel = "my_account"
	)
	w, err := NewWalletFromFile(path.Join(examplesDir, walletFile))
	require.NoError(t, err)
	require.Equal(t, 1, len(w.Accounts))
	require.Equal(t, accountLabel, w.Accounts[0].Label)
	require.NoError(t, w.Accounts[0].Decrypt(walletPass))

	// we need to keep the owner of the example contracts the same as the wallet account
	require.Equal(t, "NX1yL5wDx3inK2qUVLRVaqCLUxYnAbv85S", w.Accounts[0].Address, "need to change `owner` in the example contracts")
}
