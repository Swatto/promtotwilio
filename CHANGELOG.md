# Changelog

## [1.10.8](https://github.com/Swatto/promtotwilio/compare/v1.10.7...v1.10.8) (2026-11-14)


### Dependencies

* **docker:** update Go to 1.27.4 for stdlib security fix ([#66](https://github.com/Swatto/promtotwilio/issues/66)) ([5fec255](https://github.com/Swatto/promtotwilio/commit/5fec2550ef3092902c5a980b9b004a85cf85f8cd))

## [1.10.7](https://github.com/Swatto/promtotwilio/compare/v1.10.6...v1.10.7) (2026-11-03)


### Bug fixes

* **ci:** stop e2e failures from passing CI and remove timing flakes ([#62](https://github.com/Swatto/promtotwilio/issues/62)) ([d8f6735](https://github.com/Swatto/promtotwilio/commit/d8f6735d61ea3a578b89a548dad4dac81a2f3a54))
* correct label substitution and UTF-8 truncation; simplify handler ([5261cf8](https://github.com/Swatto/promtotwilio/commit/5261cf895e64763738d22234c7202bf54786a543))
* correct label substitution and UTF-8 truncation; simplify handler ([cae9ef0](https://github.com/Swatto/promtotwilio/commit/cae9ef0ce7b729e30ef572ffc3cac1f809679e2d))
* fail at startup on invalid RATE_LIMIT or MAX_MESSAGE_LENGTH instead of silently ignoring them ([#61](https://github.com/Swatto/promtotwilio/issues/61)) ([4e0b32b](https://github.com/Swatto/promtotwilio/commit/4e0b32be6d116a14d4d4b648aee06d288ac3618e))
* stop duplicate and unbounded SMS sends to Twilio ([#63](https://github.com/Swatto/promtotwilio/issues/63)) ([e77aeb3](https://github.com/Swatto/promtotwilio/commit/e77aeb3e471f1efc39beff5b9c77db2cbcf828af))

## [1.10.6](https://github.com/Swatto/promtotwilio/compare/v1.10.5...v1.10.6) (2026-10-26)


### Documentation

* add a security policy ([da28491](https://github.com/Swatto/promtotwilio/commit/da284910a9a97a169d55d874bb5093e3dd688a59))
* add a security policy ([cfa22d7](https://github.com/Swatto/promtotwilio/commit/cfa22d7fe3312fbd5cae3cf4d65d5618cebcac8c))

## [1.10.5](https://github.com/Swatto/promtotwilio/compare/v1.10.4...v1.10.5) (2026-10-24)


### Bug fixes

* **test:** close idle client connections before stopping the server ([d089ed0](https://github.com/Swatto/promtotwilio/commit/d089ed0b2e0aa3939d94b9838edba0fdf4c1f8b8))
