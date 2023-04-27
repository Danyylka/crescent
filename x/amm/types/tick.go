package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	utils "github.com/crescent-network/crescent/v5/types"
	exchangetypes "github.com/crescent-network/crescent/v5/x/exchange/types"
)

func SqrtPriceAtTick(tick int32, prec int) sdk.Dec {
	return utils.DecApproxSqrt(exchangetypes.PriceAtTick(tick, prec))
}

func NewTickInfo() TickInfo {
	return TickInfo{
		GrossLiquidity:    utils.ZeroDec,
		NetLiquidity:      utils.ZeroDec,
		FeeGrowthOutside0: utils.ZeroDec,
		FeeGrowthOutside1: utils.ZeroDec,
	}
}

func (tickInfo TickInfo) Validate() error {
	if tickInfo.GrossLiquidity.IsNegative() {
		return fmt.Errorf("gross liquidity must not be negative: %s", tickInfo.GrossLiquidity)
	}
	if tickInfo.NetLiquidity.IsZero() {
		return fmt.Errorf("net liquidity must not be 0")
	}
	if tickInfo.FeeGrowthOutside0.IsNegative() {
		return fmt.Errorf("fee growth outside 0 must not be negative: %s", tickInfo.FeeGrowthOutside0)
	}
	if tickInfo.FeeGrowthOutside1.IsNegative() {
		return fmt.Errorf("fee growth outside 1 must not be negative: %s", tickInfo.FeeGrowthOutside1)
	}
	return nil
}
