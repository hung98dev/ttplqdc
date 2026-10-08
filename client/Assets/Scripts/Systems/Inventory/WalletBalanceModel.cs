namespace ThinhThan.Systems.Inventory
{
    /// <summary>One wallet balance row (432 WalletBalance).</summary>
    public sealed class WalletBalanceModel
    {
        public string CurrencyId = "";
        public long Amount;
        public long Cap;
    }

}
