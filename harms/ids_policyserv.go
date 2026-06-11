package harms

// ***************************************************************************************
// **Content Warning**: This file contains identifiers for harmful content, but does not
// attempt to describe the harms in detail. This includes identifiers and titles for those
// identifiers for harms a user may encounter in a Matrix chat room.
// ---------------------------------------------------------------------------------------
// The harms defined in this file are custom to policyserv.
// ***************************************************************************************

const psIdPrefix = "org.matrix.policyserv"

const (
	// PolicyservMedia - policyserv - "Media"
	PolicyservMedia Harm = psIdPrefix + `.media`
	// PolicyservSpecNonCompliance - policyserv - "Spec Non-Compliance"
	PolicyservSpecNonCompliance Harm = psIdPrefix + `.spec_non_compliance`
)
