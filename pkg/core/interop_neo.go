package core

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	gherr "github.com/pkg/errors"
)

const (
	// MaxContractDescriptionLen is the maximum length for contract description.
	MaxContractDescriptionLen = 65536
	// MaxContractScriptSize is the maximum script size for a contract.
	MaxContractScriptSize = 1024 * 1024
	// MaxContractParametersNum is the maximum number of parameters for a contract.
	MaxContractParametersNum = 252
	// MaxContractStringLen is the maximum length for contract metadata strings.
	MaxContractStringLen = 252
	// MaxAssetNameLen is the maximum length of asset name.
	MaxAssetNameLen = 1024
	// MaxAssetPrecision is the maximum precision of asset.
	MaxAssetPrecision = 8
	// BlocksPerYear is a multiplier for asset renewal.
	BlocksPerYear = 2000000
	// DefaultAssetLifetime is the default lifetime of an asset (which differs
	// from assets created by register tx).
	DefaultAssetLifetime = 1 + BlocksPerYear
)

// headerGetVersion returns version from the header.
func headerGetVersion(ic *interop.Context, v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.Version)
	return nil
}

// headerGetMerkleRoot returns version from the header.
func headerGetMerkleRoot(ic *interop.Context, v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.MerkleRoot.BytesBE())
	return nil
}

// headerGetNextConsensus returns version from the header.
func headerGetNextConsensus(ic *interop.Context, v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.NextConsensus.BytesBE())
	return nil
}

// txGetAttributes returns current transaction attributes.
func txGetAttributes(ic *interop.Context, v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	if len(tx.Attributes) > vm.MaxArraySize {
		return errors.New("too many attributes")
	}
	attrs := make([]vm.StackItem, 0, len(tx.Attributes))
	for i := range tx.Attributes {
		attrs = append(attrs, vm.NewInteropItem(&tx.Attributes[i]))
	}
	v.Estack().PushVal(attrs)
	return nil
}

// txGetWitnesses returns current transaction witnesses.
func txGetWitnesses(ic *interop.Context, v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	if len(tx.Scripts) > vm.MaxArraySize {
		return errors.New("too many outputs")
	}
	scripts := make([]vm.StackItem, 0, len(tx.Scripts))
	for i := range tx.Scripts {
		scripts = append(scripts, vm.NewInteropItem(&tx.Scripts[i]))
	}
	v.Estack().PushVal(scripts)
	return nil
}

// witnessGetVerificationScript returns current witness' script.
func witnessGetVerificationScript(ic *interop.Context, v *vm.VM) error {
	witInterface := v.Estack().Pop().Value()
	wit, ok := witInterface.(*transaction.Witness)
	if !ok {
		return errors.New("value is not a witness")
	}
	// It's important not to share wit.VerificationScript slice with the code running in VM.
	script := make([]byte, len(wit.VerificationScript))
	copy(script, wit.VerificationScript)
	v.Estack().PushVal(script)
	return nil
}

// attrGetData returns tx attribute data.
func attrGetData(ic *interop.Context, v *vm.VM) error {
	attrInterface := v.Estack().Pop().Value()
	attr, ok := attrInterface.(*transaction.Attribute)
	if !ok {
		return fmt.Errorf("%T is not an attribute", attr)
	}
	v.Estack().PushVal(attr.Data)
	return nil
}

// attrGetData returns tx attribute usage field.
func attrGetUsage(ic *interop.Context, v *vm.VM) error {
	attrInterface := v.Estack().Pop().Value()
	attr, ok := attrInterface.(*transaction.Attribute)
	if !ok {
		return fmt.Errorf("%T is not an attribute", attr)
	}
	v.Estack().PushVal(int(attr.Usage))
	return nil
}

// bcGetAccount returns or creates an account.
func bcGetAccount(ic *interop.Context, v *vm.VM) error {
	accbytes := v.Estack().Pop().Bytes()
	acchash, err := util.Uint160DecodeBytesBE(accbytes)
	if err != nil {
		return err
	}
	acc, err := ic.DAO.GetAccountStateOrNew(acchash)
	if err != nil {
		return err
	}
	v.Estack().PushVal(vm.NewInteropItem(acc))
	return nil
}

// bcGetAsset returns an asset.
func bcGetAsset(ic *interop.Context, v *vm.VM) error {
	asbytes := v.Estack().Pop().Bytes()
	ashash, err := util.Uint256DecodeBytesBE(asbytes)
	if err != nil {
		return err
	}
	as, err := ic.DAO.GetAssetState(ashash)
	if err != nil {
		return errors.New("asset not found")
	}
	v.Estack().PushVal(vm.NewInteropItem(as))
	return nil
}

// accountGetBalance returns balance for a given account.
func accountGetBalance(ic *interop.Context, v *vm.VM) error {
	accInterface := v.Estack().Pop().Value()
	acc, ok := accInterface.(*state.Account)
	if !ok {
		return fmt.Errorf("%T is not an account state", acc)
	}
	asbytes := v.Estack().Pop().Bytes()
	ashash, err := util.Uint256DecodeBytesBE(asbytes)
	if err != nil {
		return err
	}
	balance, ok := acc.GetBalanceValues()[ashash]
	if !ok {
		balance = util.Fixed8(0)
	}
	v.Estack().PushVal(int64(balance))
	return nil
}

// accountGetScriptHash returns script hash of a given account.
func accountGetScriptHash(ic *interop.Context, v *vm.VM) error {
	accInterface := v.Estack().Pop().Value()
	acc, ok := accInterface.(*state.Account)
	if !ok {
		return fmt.Errorf("%T is not an account state", acc)
	}
	v.Estack().PushVal(acc.ScriptHash.BytesBE())
	return nil
}

// accountIsStandard checks whether given account is standard.
func accountIsStandard(ic *interop.Context, v *vm.VM) error {
	accbytes := v.Estack().Pop().Bytes()
	acchash, err := util.Uint160DecodeBytesBE(accbytes)
	if err != nil {
		return err
	}
	contract, err := ic.DAO.GetContractState(acchash)
	res := err != nil || vm.IsStandardContract(contract.Script)
	v.Estack().PushVal(res)
	return nil
}

// storageFind finds stored key-value pair.
func storageFind(ic *interop.Context, v *vm.VM) error {
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	err := checkStorageContext(ic, stc)
	if err != nil {
		return err
	}
	prefix := v.Estack().Pop().Bytes()
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(stc.ScriptHash, prefix)
	if err != nil {
		return err
	}

	filteredMap := vm.NewMapItem()
	for k, v := range siMap {
		filteredMap.Add(vm.NewByteArrayItem(append(prefix, []byte(k)...)), vm.NewByteArrayItem(v.Value))
	}
	sort.Slice(filteredMap.Value().([]vm.MapElement), func(i, j int) bool {
		return bytes.Compare(filteredMap.Value().([]vm.MapElement)[i].Key.Value().([]byte),
			filteredMap.Value().([]vm.MapElement)[j].Key.Value().([]byte)) == -1
	})

	item := vm.NewMapIterator(filteredMap)
	v.Estack().PushVal(item)

	return nil
}

// createContractStateFromVM pops all contract state elements from the VM
// evaluation stack, does a lot of checks and returns Contract if it
// succeeds.
func createContractStateFromVM(ic *interop.Context, v *vm.VM) (*state.Contract, error) {
	if ic.Trigger != trigger.Application {
		return nil, errors.New("can't create contract when not triggered by an application")
	}
	script := v.Estack().Pop().Bytes()
	if len(script) > MaxContractScriptSize {
		return nil, errors.New("the script is too big")
	}
	paramBytes := v.Estack().Pop().Bytes()
	if len(paramBytes) > MaxContractParametersNum {
		return nil, errors.New("too many parameters for a script")
	}
	paramList := make([]smartcontract.ParamType, len(paramBytes))
	for k, v := range paramBytes {
		paramList[k] = smartcontract.ParamType(v)
	}
	retType := smartcontract.ParamType(v.Estack().Pop().BigInt().Int64())
	properties := smartcontract.PropertyState(v.Estack().Pop().BigInt().Int64())
	name := v.Estack().Pop().Bytes()
	if len(name) > MaxContractStringLen {
		return nil, errors.New("too big name")
	}
	version := v.Estack().Pop().Bytes()
	if len(version) > MaxContractStringLen {
		return nil, errors.New("too big version")
	}
	author := v.Estack().Pop().Bytes()
	if len(author) > MaxContractStringLen {
		return nil, errors.New("too big author")
	}
	email := v.Estack().Pop().Bytes()
	if len(email) > MaxContractStringLen {
		return nil, errors.New("too big email")
	}
	desc := v.Estack().Pop().Bytes()
	if len(desc) > MaxContractDescriptionLen {
		return nil, errors.New("too big description")
	}
	contract := &state.Contract{
		Script:      script,
		ParamList:   paramList,
		ReturnType:  retType,
		Properties:  properties,
		Name:        string(name),
		CodeVersion: string(version),
		Author:      string(author),
		Email:       string(email),
		Description: string(desc),
	}
	return contract, nil
}

// contractCreate creates a contract.
func contractCreate(ic *interop.Context, v *vm.VM) error {
	newcontract, err := createContractStateFromVM(ic, v)
	if err != nil {
		return err
	}
	contract, err := ic.DAO.GetContractState(newcontract.ScriptHash())
	if err != nil {
		contract = newcontract
		err := ic.DAO.PutContractState(contract)
		if err != nil {
			return err
		}
	}
	v.Estack().PushVal(vm.NewInteropItem(contract))
	return nil
}

// contractGetScript returns a script associated with a contract.
func contractGetScript(ic *interop.Context, v *vm.VM) error {
	csInterface := v.Estack().Pop().Value()
	cs, ok := csInterface.(*state.Contract)
	if !ok {
		return fmt.Errorf("%T is not a contract state", cs)
	}
	v.Estack().PushVal(cs.Script)
	return nil
}

// contractIsPayable returns whether contract is payable.
func contractIsPayable(ic *interop.Context, v *vm.VM) error {
	csInterface := v.Estack().Pop().Value()
	cs, ok := csInterface.(*state.Contract)
	if !ok {
		return fmt.Errorf("%T is not a contract state", cs)
	}
	v.Estack().PushVal(cs.IsPayable())
	return nil
}

// contractMigrate migrates a contract.
func contractMigrate(ic *interop.Context, v *vm.VM) error {
	newcontract, err := createContractStateFromVM(ic, v)
	if err != nil {
		return err
	}
	contract, err := ic.DAO.GetContractState(newcontract.ScriptHash())
	if err != nil {
		contract = newcontract
		err := ic.DAO.PutContractState(contract)
		if err != nil {
			return err
		}
		if contract.HasStorage() {
			hash := v.GetCurrentScriptHash()
			siMap, err := ic.DAO.GetStorageItems(hash)
			if err != nil {
				return err
			}
			for k, v := range siMap {
				v.IsConst = false
				err = ic.DAO.PutStorageItem(contract.ScriptHash(), []byte(k), v)
				if err != nil {
					return err
				}
			}
		}
	}
	v.Estack().PushVal(vm.NewInteropItem(contract))
	return contractDestroy(ic, v)
}

// assetCreate creates an asset.
func assetCreate(ic *interop.Context, v *vm.VM) error {
	if ic.Trigger != trigger.Application {
		return errors.New("can't create asset when not triggered by an application")
	}
	atype := transaction.AssetType(v.Estack().Pop().BigInt().Int64())
	switch atype {
	case transaction.Currency, transaction.Share, transaction.Invoice, transaction.Token:
		// ok
	default:
		return fmt.Errorf("wrong asset type: %x", atype)
	}
	name := string(v.Estack().Pop().Bytes())
	if len(name) > MaxAssetNameLen {
		return errors.New("too big name")
	}
	amount := util.Fixed8(v.Estack().Pop().BigInt().Int64())
	if amount == util.Fixed8(0) {
		return errors.New("asset amount can't be zero")
	}
	if amount < -util.Satoshi() {
		return errors.New("asset amount can't be negative (except special -Satoshi value")
	}
	if atype == transaction.Invoice && amount != -util.Satoshi() {
		return errors.New("invoice assets can only have -Satoshi amount")
	}
	precision := byte(v.Estack().Pop().BigInt().Int64())
	if precision > MaxAssetPrecision {
		return fmt.Errorf("can't have asset precision of more than %d", MaxAssetPrecision)
	}
	if atype == transaction.Share && precision != 0 {
		return errors.New("share assets can only have zero precision")
	}
	if amount != -util.Satoshi() && (int64(amount)%int64(math.Pow10(int(MaxAssetPrecision-precision))) != 0) {
		return errors.New("given asset amount has fractional component")
	}
	owner, err := keys.NewPublicKeyFromBytes(v.Estack().Pop().Bytes())
	if err != nil {
		return gherr.Wrap(err, "failed to get owner key")
	}
	if owner.IsInfinity() {
		return errors.New("can't have infinity as an owner key")
	}
	witnessOk, err := runtime.CheckKeyedWitness(ic, v, owner)
	if err != nil {
		return err
	}
	if !witnessOk {
		return errors.New("witness check didn't succeed")
	}
	admin, err := util.Uint160DecodeBytesBE(v.Estack().Pop().Bytes())
	if err != nil {
		return gherr.Wrap(err, "failed to get admin")
	}
	issuer, err := util.Uint160DecodeBytesBE(v.Estack().Pop().Bytes())
	if err != nil {
		return gherr.Wrap(err, "failed to get issuer")
	}
	asset := &state.Asset{
		ID:         ic.Tx.Hash(),
		AssetType:  atype,
		Name:       name,
		Amount:     amount,
		Precision:  precision,
		Owner:      *owner,
		Admin:      admin,
		Issuer:     issuer,
		Expiration: ic.Chain.BlockHeight() + DefaultAssetLifetime,
	}
	err = ic.DAO.PutAssetState(asset)
	if err != nil {
		return gherr.Wrap(err, "failed to Store asset")
	}
	v.Estack().PushVal(vm.NewInteropItem(asset))
	return nil
}

// assetGetAdmin returns asset admin.
func assetGetAdmin(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.Admin.BytesBE())
	return nil
}

// assetGetAmount returns the overall amount of asset available.
func assetGetAmount(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int64(as.Amount))
	return nil
}

// assetGetAssetId returns the id of an asset.
func assetGetAssetID(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.ID.BytesBE())
	return nil
}

// assetGetAssetType returns type of an asset.
func assetGetAssetType(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int(as.AssetType))
	return nil
}

// assetGetAvailable returns available (not yet issued) amount of asset.
func assetGetAvailable(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int(as.Available))
	return nil
}

// assetGetIssuer returns issuer of an asset.
func assetGetIssuer(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.Issuer.BytesBE())
	return nil
}

// assetGetOwner returns owner of an asset.
func assetGetOwner(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.Owner.Bytes())
	return nil
}

// assetGetPrecision returns precision used to measure this asset.
func assetGetPrecision(ic *interop.Context, v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int(as.Precision))
	return nil
}

// assetRenew updates asset expiration date.
func assetRenew(ic *interop.Context, v *vm.VM) error {
	if ic.Trigger != trigger.Application {
		return errors.New("can't create asset when not triggered by an application")
	}
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*state.Asset)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	years := byte(v.Estack().Pop().BigInt().Int64())
	// Not sure why C# code regets an asset from the Store, but we also do it.
	asset, err := ic.DAO.GetAssetState(as.ID)
	if err != nil {
		return errors.New("can't renew non-existent asset")
	}
	if asset.Expiration < ic.Chain.BlockHeight()+1 {
		asset.Expiration = ic.Chain.BlockHeight() + 1
	}
	expiration := uint64(asset.Expiration) + uint64(years)*BlocksPerYear
	if expiration > math.MaxUint32 {
		expiration = math.MaxUint32
	}
	asset.Expiration = uint32(expiration)
	err = ic.DAO.PutAssetState(asset)
	if err != nil {
		return gherr.Wrap(err, "failed to Store asset")
	}
	v.Estack().PushVal(expiration)
	return nil
}

// runtimeSerialize serializes top stack item into a ByteArray.
func runtimeSerialize(_ *interop.Context, v *vm.VM) error {
	return vm.RuntimeSerialize(v)
}

// runtimeDeserialize deserializes ByteArray from a stack into an item.
func runtimeDeserialize(_ *interop.Context, v *vm.VM) error {
	return vm.RuntimeDeserialize(v)
}
