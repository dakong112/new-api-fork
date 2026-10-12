package service

import "time"

// SSOCodeTTL bounds how long an application has to redeem a login code.
const SSOCodeTTL = 60 * time.Second
