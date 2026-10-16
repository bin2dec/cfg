package cfg

// Mask is the placeholder substituted for a secret, or for a secret part of a larger value
// (such as the password in a URL), whenever a Secret or SecretURL is rendered. The value may be
// changed to customize the placeholder; set it once at startup, before concurrent use, since it
// is read without synchronization.
var Mask = "REDACTED"
