package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramstypes.ParamSet = (*Params)(nil)

var (
	KeySpotMarketCreationFee = []byte("SpotMarketCreationFee")
)

var (
	DefaultSpotMarketCreationFee = sdk.NewCoins()

	MinPrice = sdk.NewDecWithPrec(1, 14)
	MaxPrice = sdk.NewDecFromInt(sdk.NewIntWithDecimal(1, 40))
)

func ParamKeyTable() paramstypes.KeyTable {
	return paramstypes.NewKeyTable().RegisterParamSet(&Params{})
}

// DefaultParams returns a default params for the module.
func DefaultParams() Params {
	return Params{
		SpotMarketCreationFee: DefaultSpotMarketCreationFee,
	}
}

// ParamSetPairs implements ParamSet.
func (params *Params) ParamSetPairs() paramstypes.ParamSetPairs {
	return paramstypes.ParamSetPairs{
		paramstypes.NewParamSetPair(KeySpotMarketCreationFee, &params.SpotMarketCreationFee, validateSpotMarketCreationFee),
	}
}

// Validate validates Params.
func (params Params) Validate() error {
	for _, field := range []struct {
		val          interface{}
		validateFunc func(i interface{}) error
	}{
		{params.SpotMarketCreationFee, validateSpotMarketCreationFee},
	} {
		if err := field.validateFunc(field.val); err != nil {
			return err
		}
	}
	return nil
}

func validateSpotMarketCreationFee(i interface{}) error {
	v, ok := i.(sdk.Coins)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if err := v.Validate(); err != nil {
		return fmt.Errorf("invalid spot market creation fee: %w", err)
	}
	return nil
}
