package keeper_test

import (
	"github.com/cosmosquad-labs/squad/x/liquidity/types"
)

func (s *KeeperTestSuite) TestDefaultGenesis() {
	genState := *types.DefaultGenesis()

	s.keeper.InitGenesis(s.ctx, genState)
	got := s.keeper.ExportGenesis(s.ctx)
	s.Require().Equal(genState, *got)
}

func (s *KeeperTestSuite) TestImportExportGenesis() {
	s.ctx = s.ctx.WithBlockHeight(1).WithBlockTime(parseTime("2022-01-01T00:00:00Z"))
	k, ctx := s.keeper, s.ctx

	pair := s.createPair(s.addr(0), "denom1", "denom2", true)
	pool := s.createPool(s.addr(0), pair.Id, parseCoins("1000000denom1,1000000denom2"), true)

	s.depositBatch(s.addr(1), pool.Id, parseCoins("1000000denom1,1000000denom2"), true)
	s.nextBlock()

	poolCoin := s.getBalance(s.addr(1), pool.PoolCoinDenom)
	poolCoin.Amount = poolCoin.Amount.QuoRaw(2)
	s.withdrawBatch(s.addr(1), pool.Id, poolCoin)
	s.nextBlock()

	s.buyLimitOrderBatch(s.addr(2), pair.Id, parseDec("1.0"), newInt(10000), 0, true)
	s.nextBlock()

	depositReq := s.depositBatch(s.addr(3), pool.Id, parseCoins("1000000denom1,1000000denom2"), true)
	withdrawReq := s.withdrawBatch(s.addr(1), pool.Id, poolCoin)
	swapReq := s.sellLimitOrderBatch(s.addr(3), pair.Id, parseDec("1.0"), newInt(1000), 0, true)

	genState := k.ExportGenesis(ctx)

	bz := s.app.AppCodec().MustMarshalJSON(genState)

	s.SetupTest()
	s.ctx = s.ctx.WithBlockHeight(1).WithBlockTime(parseTime("2022-01-01T00:00:00Z"))
	k, ctx = s.keeper, s.ctx

	var genState2 types.GenesisState
	s.app.AppCodec().MustUnmarshalJSON(bz, &genState2)
	k.InitGenesis(ctx, genState2)
	genState3 := k.ExportGenesis(ctx)

	s.Require().Equal(*genState, *genState3)

	depositReq2, found := k.GetDepositRequest(ctx, depositReq.PoolId, depositReq.Id)
	s.Require().True(found)
	s.Require().Equal(depositReq, depositReq2)
	withdrawReq2, found := k.GetWithdrawRequest(ctx, withdrawReq.PoolId, withdrawReq.Id)
	s.Require().True(found)
	s.Require().Equal(withdrawReq, withdrawReq2)
	swapReq2, found := k.GetSwapRequest(ctx, swapReq.PairId, swapReq.Id)
	s.Require().True(found)
	s.Require().Equal(swapReq, swapReq2)
}
