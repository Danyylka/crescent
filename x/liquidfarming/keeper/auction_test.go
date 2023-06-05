package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	utils "github.com/crescent-network/crescent/v5/types"
	"github.com/crescent-network/crescent/v5/x/liquidfarming/keeper"
	"github.com/crescent-network/crescent/v5/x/liquidfarming/types"
)

func (s *KeeperTestSuite) TestPlaceBid() {
	liquidFarm := s.CreateSampleLiquidFarm()

	minterAddr1 := utils.TestAddress(1)
	s.MintShare(minterAddr1, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.NextBlock()

	s.AdvanceRewardsAuctions()

	auction, found := s.keeper.GetLastRewardsAuction(s.Ctx, liquidFarm.Id)
	s.Require().True(found)

	bidderAddr1 := utils.TestAddress(2)
	s.MintShare(bidderAddr1, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, utils.ParseCoin("100000lfshare1"))
	s.NextBlock()

	s.AdvanceRewardsAuctions()

	auction, _ = s.keeper.GetLastRewardsAuction(s.Ctx, liquidFarm.Id)

	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, utils.ParseCoin("100000lfshare1"))

	bidderAddr2 := utils.TestAddress(3)
	s.MintShare(bidderAddr2, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.PlaceBid(bidderAddr2, liquidFarm.Id, auction.Id, utils.ParseCoin("200000lfshare1"))
	s.NextBlock()

	s.Require().Len(s.keeper.GetAllBids(s.Ctx), 2)

	bidderAddr3 := utils.TestAddress(4)
	s.MintShare(bidderAddr3, liquidFarm.Id, utils.ParseCoins("1000_000000ucre,5000_000000uusd"), true)

	for _, tc := range []struct {
		name        string
		msg         *types.MsgPlaceBid
		expectedErr string
	}{
		{
			"happy case",
			types.NewMsgPlaceBid(
				bidderAddr3, liquidFarm.Id, auction.Id, utils.ParseCoin("300000lfshare1")),
			"",
		},
		{
			"minimum bid amount",
			types.NewMsgPlaceBid(
				bidderAddr3, liquidFarm.Id, auction.Id, utils.ParseCoin("100lfshare1")),
			"share amount must not be smaller than 10000: invalid request",
		},
		{
			"smaller than winning bid",
			types.NewMsgPlaceBid(
				bidderAddr3, liquidFarm.Id, auction.Id, utils.ParseCoin("150000lfshare1")),
			"share amount must be greater than winning bid's share 200000: insufficient bid amount",
		},
		{
			"finished auction",
			types.NewMsgPlaceBid(
				bidderAddr3, liquidFarm.Id, auction.Id-1, utils.ParseCoin("300000lfshare1")),
			"rewards auction is not started: invalid request",
		},
	} {
		s.Run(tc.name, func() {
			s.Require().NoError(tc.msg.ValidateBasic())
			cacheCtx, _ := s.Ctx.CacheContext()
			_, err := keeper.NewMsgServerImpl(s.keeper).PlaceBid(sdk.WrapSDKContext(cacheCtx), tc.msg)
			if tc.expectedErr == "" {
				s.Require().NoError(err)
			} else {
				s.Require().EqualError(err, tc.expectedErr)
			}
		})
	}
}

func (s *KeeperTestSuite) TestRewardsAuction() {
	liquidFarm := s.CreateSampleLiquidFarm()

	minterAddr := utils.TestAddress(1)
	s.MintShare(minterAddr, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.NextBlock()

	// Start the first rewards auction.
	s.AdvanceRewardsAuctions()

	position := s.App.LiquidFarmingKeeper.MustGetLiquidFarmPosition(s.Ctx, liquidFarm)
	rewards, err := s.App.AMMKeeper.CollectibleCoins(s.Ctx, position.Id)
	s.Require().NoError(err)
	s.Require().Equal("5786uatom", rewards.String())

	bidderAddr1 := utils.TestAddress(2)
	bidderShare1, _, _, _ := s.MintShare(bidderAddr1, liquidFarm.Id, utils.ParseCoins("10_00000ucre,50_000000uusd"), true)
	bidderAddr2 := utils.TestAddress(3)
	bidderShare2, _, _, _ := s.MintShare(bidderAddr2, liquidFarm.Id, utils.ParseCoins("20_00000ucre,100_000000uusd"), true)

	auction, found := s.App.LiquidFarmingKeeper.GetLastRewardsAuction(s.Ctx, liquidFarm.Id)
	s.Require().True(found)
	s.Require().Nil(auction.WinningBid)
	s.Require().Equal(types.AuctionStatusStarted, auction.Status)

	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, bidderShare1.SubAmount(sdk.NewInt(1000)))
	auction, _ = s.App.LiquidFarmingKeeper.GetRewardsAuction(s.Ctx, liquidFarm.Id, auction.Id)
	s.Require().Equal(bidderAddr1.String(), auction.WinningBid.Bidder)

	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, bidderShare1) // Update the bid with the higher amount
	auction, _ = s.App.LiquidFarmingKeeper.GetRewardsAuction(s.Ctx, liquidFarm.Id, auction.Id)
	s.Require().Equal(bidderShare1, auction.WinningBid.Share)

	s.PlaceBid(bidderAddr2, liquidFarm.Id, auction.Id, bidderShare2)
	auction, _ = s.App.LiquidFarmingKeeper.GetRewardsAuction(s.Ctx, liquidFarm.Id, auction.Id)
	s.Require().Equal(bidderAddr2.String(), auction.WinningBid.Bidder)
	s.Require().Equal(bidderShare2, auction.WinningBid.Share)

	// Finish the current rewards auction.
	s.AdvanceRewardsAuctions()
	s.Require().Equal(sdk.NewInt(5768), s.GetAllBalances(bidderAddr2).AmountOf("uatom"))

	auction, _ = s.App.LiquidFarmingKeeper.GetRewardsAuction(s.Ctx, liquidFarm.Id, auction.Id)
	s.Require().Equal(types.AuctionStatusFinished, auction.Status)
	s.Require().Equal("5786uatom", auction.Rewards.String()) // Rewards before deducting fees
	s.Require().Equal("18uatom", auction.Fees.String())
}

func (s *KeeperTestSuite) TestPlaceBid_Refund() {
	liquidFarm := s.CreateSampleLiquidFarm()

	minterAddr1 := utils.TestAddress(1)
	s.MintShare(minterAddr1, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.NextBlock()

	s.AdvanceRewardsAuctions()

	bidderAddr1 := utils.TestAddress(2)
	s.MintShare(bidderAddr1, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)

	auction, found := s.keeper.GetLastRewardsAuction(s.Ctx, liquidFarm.Id)
	s.Require().True(found)
	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, utils.ParseCoin("100000lfshare1"))
	s.NextBlock()

	balancesBefore := s.GetAllBalances(bidderAddr1)
	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, utils.ParseCoin("200000lfshare1"))
	s.NextBlock()
	balancesAfter := s.GetAllBalances(bidderAddr1)
	s.Require().Equal("100000lfshare1", balancesBefore.Sub(balancesAfter).String())
}

func (s *KeeperTestSuite) TestAfterRewardsAllocated() {
	liquidFarm := s.CreateSampleLiquidFarm()

	minterAddr := utils.TestAddress(1)
	_, _, liquidity, _ := s.MintShare(minterAddr, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.NextBlock()

	s.AdvanceRewardsAuctions()

	// Ensure that the rewards auction is created
	auction, found := s.keeper.GetLastRewardsAuction(s.Ctx, liquidFarm.Id)
	s.Require().True(found)
	s.Require().Equal(types.AuctionStatusStarted, auction.Status)

	bidderAddr1 := utils.TestAddress(2)
	bidderAddr2 := utils.TestAddress(3)
	bidderAddr3 := utils.TestAddress(4)
	s.MintShare(bidderAddr1, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.MintShare(bidderAddr2, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.MintShare(bidderAddr3, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	// Previous share balance
	s.Require().Equal("4357388321lfshare1", s.GetBalance(bidderAddr1, "lfshare1").String())
	s.Require().Equal("4357388321lfshare1", s.GetBalance(bidderAddr2, "lfshare1").String())
	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, utils.ParseCoin("100000lfshare1"))
	s.PlaceBid(bidderAddr2, liquidFarm.Id, auction.Id, utils.ParseCoin("200000lfshare1"))
	s.PlaceBid(bidderAddr3, liquidFarm.Id, auction.Id, utils.ParseCoin("300000lfshare1"))

	s.NextBlock()
	s.AdvanceRewardsAuctions()

	// Ensure that two bidders got their shares back to their balances
	s.Require().Equal("4357388321lfshare1", s.GetBalance(bidderAddr1, "lfshare1").String())
	s.Require().Equal("4357388321lfshare1", s.GetBalance(bidderAddr2, "lfshare1").String())
	s.Require().True(s.GetBalance(bidderAddr3, "uatom").Amount.GT(sdk.NewInt(1)))

	// One more epoch should be advanced
	s.NextBlock()
	s.AdvanceRewardsAuctions()

	// Ensure liquidity per share increased due to the auction result
	removedLiquidity, _, _ := s.BurnShare(minterAddr, liquidFarm.Id, s.GetBalance(minterAddr, "lfshare1"))
	s.Require().True(removedLiquidity.GT(liquidity))
}

func (s *KeeperTestSuite) TestAuctionSkipped() {
	liquidFarm := s.CreateSampleLiquidFarm()

	minterAddr := utils.TestAddress(1)
	s.MintShare(minterAddr, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)

	s.NextBlock()
	s.AdvanceRewardsAuctions()

	auction, found := s.keeper.GetLastRewardsAuction(s.Ctx, liquidFarm.Id)
	s.Require().True(found)
	s.Require().Equal(types.AuctionStatusStarted, auction.Status)

	s.AdvanceRewardsAuctions()

	auction, found = s.keeper.GetRewardsAuction(s.Ctx, liquidFarm.Id, auction.Id)
	s.Require().True(found)
	s.Require().Equal(types.AuctionStatusSkipped, auction.Status)
}

func (s *KeeperTestSuite) TestRewardsAuction_RewardsAndFees() {
	liquidFarm := s.CreateSampleLiquidFarm()
	s.NextBlock()

	minterAddr := utils.TestAddress(1)
	s.MintShare(minterAddr, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	s.NextBlock()

	s.AdvanceRewardsAuctions()

	bidderAddr1 := utils.TestAddress(2)
	s.MintShare(bidderAddr1, liquidFarm.Id, utils.ParseCoins("100_000000ucre,500_000000uusd"), true)
	auction, _ := s.keeper.GetLastRewardsAuction(s.Ctx, liquidFarm.Id)
	s.PlaceBid(bidderAddr1, liquidFarm.Id, auction.Id, utils.ParseCoin("100000lfshare1"))
	s.NextBlock()

	position := s.keeper.MustGetLiquidFarmPosition(s.Ctx, liquidFarm)
	rewards, err := s.App.AMMKeeper.CollectibleCoins(s.Ctx, position.Id)
	s.Require().NoError(err)

	deducted, fees := types.DeductFees(rewards, liquidFarm.FeeRate)

	s.AdvanceRewardsAuctions()

	auction, _ = s.keeper.GetRewardsAuction(s.Ctx, liquidFarm.Id, auction.Id)
	s.Require().True(auction.Rewards.IsEqual(deducted.Add(fees...)))
	s.Require().True(auction.Fees.IsEqual(fees))
}
