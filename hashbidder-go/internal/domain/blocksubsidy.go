package domain

func BlockSubsidy(height BlockHeight) Sats {
	return Sats(InitialSubsidySats >> (height.Value / HalvingInterval))
}
