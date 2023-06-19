package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	utils "github.com/crescent-network/crescent/v5/types"
	"github.com/crescent-network/crescent/v5/x/exchange/types"
)

// TODO: refactor code
func (k Keeper) RunBatchMatching(ctx sdk.Context, market types.Market) (err error) {
	// Find the best buy(bid) and sell(ask) prices to limit the price to load
	// on the other side.
	var bestBuyPrice, bestSellPrice sdk.Dec
	k.IterateOrderBookSide(ctx, market.Id, true, func(order types.Order) (stop bool) {
		bestBuyPrice = order.Price
		return true
	})
	k.IterateOrderBookSide(ctx, market.Id, false, func(order types.Order) (stop bool) {
		bestSellPrice = order.Price
		return true
	})

	// No need to run matching since the order book is not crossed.
	if !bestBuyPrice.IsNil() && !bestSellPrice.IsNil() && bestBuyPrice.LT(bestSellPrice) {
		return nil
	}

	// Construct TempOrderBookSides with the price limits we obtained previously.
	var buyObs, sellObs *types.TempOrderBookSide
	if !bestSellPrice.IsNil() {
		buyObs = k.ConstructTempOrderBookSide(ctx, market, true, &bestSellPrice, nil, nil)
	} else {
		buyObs = types.NewTempOrderBookSide(true)
	}
	if !bestBuyPrice.IsNil() {
		sellObs = k.ConstructTempOrderBookSide(ctx, market, false, &bestBuyPrice, nil, nil)
	} else {
		sellObs = types.NewTempOrderBookSide(false)
	}

	marketState := k.MustGetMarketState(ctx, market.Id)
	var lastPrice sdk.Dec
	defer func() {
		// If there was an error, exit early.
		if err != nil {
			return
		}
		// If there was no matching, exit early, too.
		if lastPrice.IsNil() {
			return
		}

		// Apply the match results.
		var tempOrders []*types.TempOrder
		for _, level := range buyObs.Levels {
			tempOrders = append(tempOrders, level.Orders...)
		}
		for _, level := range sellObs.Levels {
			tempOrders = append(tempOrders, level.Orders...)
		}
		if err = k.FinalizeMatching(ctx, market, tempOrders); err != nil {
			return
		}
		if marketState.LastPrice == nil || !marketState.LastPrice.Equal(lastPrice) {
			marketState.LastPrice = &lastPrice
			k.SetMarketState(ctx, market.Id, marketState)
		}
	}()

	if marketState.LastPrice == nil {
		// If there's no last price, then match orders at a single price.
		// The price will be the fairest price for each buy and sell orders.
		buyLevelIdx, sellLevelIdx := 0, 0
		var buyLastPrice, sellLastPrice sdk.Dec
		for buyLevelIdx < len(buyObs.Levels) && sellLevelIdx < len(sellObs.Levels) {
			buyLevel := buyObs.Levels[buyLevelIdx]
			sellLevel := sellObs.Levels[sellLevelIdx]
			if buyLevel.Price.LT(sellLevel.Price) {
				break
			}
			buyExecutableQty := types.TotalExecutableQuantity(buyLevel.Orders, buyLevel.Price)
			sellExecutableQty := types.TotalExecutableQuantity(sellLevel.Orders, sellLevel.Price)
			execQty := utils.MinInt(buyExecutableQty, sellExecutableQty)
			buyLastPrice = buyLevel.Price
			sellLastPrice = sellLevel.Price
			buyFull := execQty.Equal(buyExecutableQty)
			sellFull := execQty.Equal(sellExecutableQty)
			if buyFull {
				buyLevelIdx++
			}
			if sellFull {
				sellLevelIdx++
			}
		}
		if !buyLastPrice.IsNil() && !sellLastPrice.IsNil() {
			matchPrice := types.RoundPrice(buyLastPrice.Add(sellLastPrice).QuoInt64(2))
			buyLevelIdx, sellLevelIdx = 0, 0
			for buyLevelIdx < len(buyObs.Levels) && sellLevelIdx < len(sellObs.Levels) {
				buyLevel := buyObs.Levels[buyLevelIdx]
				sellLevel := sellObs.Levels[sellLevelIdx]
				if buyLevel.Price.LT(matchPrice) || sellLevel.Price.GT(matchPrice) {
					break
				}
				// Both sides are taker
				_, sellFull, buyFull := market.MatchOrderBookLevels(sellLevel, false, buyLevel, false, matchPrice)
				if buyFull {
					buyLevelIdx++
				}
				if sellFull {
					sellLevelIdx++
				}
			}
			lastPrice = matchPrice
		}
		return nil
	}
	lastPrice = *marketState.LastPrice

	// Phase 1: Match orders with price below(or equal to) the last price and
	// price above(or equal to) the last price.
	// The execution price is the last price.
	matchPrice := lastPrice
	buyLevelIdx, sellLevelIdx := 0, 0
	for buyLevelIdx < len(buyObs.Levels) && sellLevelIdx < len(sellObs.Levels) {
		buyLevel := buyObs.Levels[buyLevelIdx]
		sellLevel := sellObs.Levels[sellLevelIdx]
		if buyLevel.Price.LT(matchPrice) || sellLevel.Price.GT(matchPrice) {
			break
		}
		// Both sides are taker
		_, sellFull, buyFull := market.MatchOrderBookLevels(sellLevel, false, buyLevel, false, matchPrice)
		lastPrice = matchPrice
		if buyFull {
			buyLevelIdx++
		}
		if sellFull {
			sellLevelIdx++
		}
	}
	// If there's no level to match, return earlier.
	if buyLevelIdx >= len(buyObs.Levels) || sellLevelIdx >= len(sellObs.Levels) {
		return nil
	}

	// Phase 2: Match orders in traditional exchange's manner.
	// The matching price is determined by the direction of price.

	// No sell orders with price below(or equal to) the last price,
	// thus the price will increase.
	isPriceIncreasing := sellObs.Levels[sellLevelIdx].Price.GT(lastPrice)
	for buyLevelIdx < len(buyObs.Levels) && sellLevelIdx < len(sellObs.Levels) {
		buyLevel := buyObs.Levels[buyLevelIdx]
		sellLevel := sellObs.Levels[sellLevelIdx]
		if buyLevel.Price.LT(sellLevel.Price) {
			break
		}
		var price sdk.Dec
		if isPriceIncreasing {
			price = sellLevel.Price
		} else {
			price = buyLevel.Price
		}
		_, sellFull, buyFull := market.MatchOrderBookLevels(sellLevel, isPriceIncreasing, buyLevel, !isPriceIncreasing, price)
		lastPrice = price
		if buyFull {
			buyLevelIdx++
		}
		if sellFull {
			sellLevelIdx++
		}
	}

	return nil
}
