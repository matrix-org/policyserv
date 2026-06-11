package harms

// ***************************************************************************************
// **Content Warning**: This file contains identifiers for harmful content, but does not
// attempt to describe the harms in detail. This includes identifiers and titles for those
// identifiers for child safety, sexual abuse, self-harm, and other types of harm a user
// may encounter on the open internet.
// ---------------------------------------------------------------------------------------
// The harms defined in this file are defined by MSC4456.
// https://github.com/matrix-org/matrix-spec-proposals/pull/4456
// ***************************************************************************************

const idPrefix = "org.matrix.msc4456"

const (
	// SpamGeneral - Spam - "General/Other"
	SpamGeneral Harm = idPrefix + `.spam`
	// SpamFraud - Spam - "Fraud/Phishing"
	SpamFraud Harm = idPrefix + `.spam.fraud`
	// SpamImpersonation - Spam - "Impersonation"
	SpamImpersonation Harm = idPrefix + `.spam.impersonation`
	// SpamElectionInterference - Spam - "Election Interference"
	SpamElectionInterference Harm = idPrefix + `.spam.election_interference`
	// SpamFlooding - Spam - "Flooding"
	SpamFlooding Harm = idPrefix + `.spam.flooding`
)

const (
	// AdultGeneral - Adult - "General/Other"
	AdultGeneral Harm = idPrefix + `.adult`
	// AdultSexualAbuse - Adult - "Sexual Abuse"
	AdultSexualAbuse Harm = idPrefix + `.adult.sexual_abuse`
	// AdultNCII - Adult - "Non-Consensual Intimate Imagery (NCII)"
	AdultNCII Harm = idPrefix + `.adult.ncii`
	// AdultDeepfake - Adult - "Deepfake"
	AdultDeepfake Harm = idPrefix + `.adult.deepfake`
	// AdultAnimalSexualAbuse - Adult - "Animal Sexual Abuse"
	AdultAnimalSexualAbuse Harm = idPrefix + `.adult.animal_sexual_abuse`
	// AdultSexualViolence - Adult - "Sexual Violence"
	AdultSexualViolence Harm = idPrefix + `.adult.sexual_violence`
)

const (
	// HarassmentGeneral - Harassment - "General/Other"
	HarassmentGeneral Harm = idPrefix + `.harassment`
	// HarassmentTrolling - Harassment - "Trolling"
	HarassmentTrolling Harm = idPrefix + `.harassment.trolling`
	// HarassmentTargeted - Harassment - "Targeted"
	HarassmentTargeted Harm = idPrefix + `.harassment.targeted`
	// HarassmentHate - Harassment - "Hate"
	HarassmentHate Harm = idPrefix + `.harassment.hate`
	// HarassmentDoxxing - Harassment - "Doxxing/Personal Information"
	HarassmentDoxxing Harm = idPrefix + `.harassment.doxxing`
)

const (
	// ViolenceGeneral - Violence - "General/Other"
	ViolenceGeneral Harm = idPrefix + `.violence`
	// ViolenceAnimalWelfare - Violence - "Animal Welfare"
	ViolenceAnimalWelfare Harm = idPrefix + `.violence.animal_welfare`
	// ViolenceThreats - Violence - "Threats/Threatening"
	ViolenceThreats Harm = idPrefix + `.violence.threats`
	// ViolenceGraphic - Violence - "Graphic/Gore"
	ViolenceGraphic Harm = idPrefix + `.violence.graphic`
	// ViolenceGlorification - Violence - "Glorification/Promotion"
	ViolenceGlorification Harm = idPrefix + `.violence.glorification`
	// ViolenceExtremist - Violence - "Extremism"
	ViolenceExtremist Harm = idPrefix + `.violence.extremist`
	// ViolenceHumanTrafficking - Violence - "Human Trafficking"
	ViolenceHumanTrafficking Harm = idPrefix + `.violence.human_trafficking`
	// ViolenceDomestic - Violence - "Domestic/Intimate Partner"
	ViolenceDomestic Harm = idPrefix + `.violence.domestic`
)

const (
	// ChildSafetyGeneral - Child Safety - "General/Other"
	ChildSafetyGeneral Harm = idPrefix + `.child_safety`
	// ChildSafetyCSAM - Child Safety - "Child Sexual Abuse Material (CSAM)"
	ChildSafetyCSAM Harm = idPrefix + `.child_safety.csam`
	// ChildSafetyGrooming - Child Safety - "Grooming"
	ChildSafetyGrooming Harm = idPrefix + `.child_safety.grooming`
	// ChildSafetyPrivacyViolation - Child Safety - "Privacy"
	ChildSafetyPrivacyViolation Harm = idPrefix + `.child_safety.privacy_violation`
	// ChildSafetyHarassment - Child Safety - "Harassment"
	ChildSafetyHarassment Harm = idPrefix + `.child_safety.harassment`
)

const (
	// DangerGeneral - Danger - "General/Other"
	DangerGeneral Harm = idPrefix + `.danger`
	// DangerSelfHarm - Danger - "Self Harm"
	DangerSelfHarm Harm = idPrefix + `.danger.self_harm`
	// DangerEatingDisorder - Danger - "Eating Disorder"
	DangerEatingDisorder Harm = idPrefix + `.danger.eating_disorder`
	// DangerChallenges - Danger - "Challenges, including Social Media Challenges"
	DangerChallenges Harm = idPrefix + `.danger.challenges`
	// DangerSubstanceAbuse - Danger - "Substance Abuse"
	DangerSubstanceAbuse Harm = idPrefix + `.danger.substance_abuse`
)

const (
	// TOSGeneral - Terms of Service - "General/Other"
	TOSGeneral Harm = idPrefix + `.tos`
	// TOSHacking - Terms of Service - "Hacking/Computer Misuse"
	TOSHacking Harm = idPrefix + `.tos.hacking`
	// TOSProhibited - Terms of Service - "Prohibited Items (Drugs, Weapons, etc)"
	TOSProhibited Harm = idPrefix + `.tos.prohibited`
	// TOSBanEvasion - Terms of Service - "Ban Evasion"
	TOSBanEvasion Harm = idPrefix + `.tos.ban_evasion`
)

const (
	// OtherGeneral - Other - "Other Concern"
	OtherGeneral Harm = idPrefix + `.other`
)
