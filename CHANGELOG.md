# Changelog

## [0.1.4](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.3...v0.1.4) (2026-03-23)


### Bug Fixes

* add sherpa-onnx libs and claude-cli to Docker image ([8bcc0d3](https://github.com/zackpollard/discord-rpg-summariser/commit/8bcc0d359dc7eccea0f1c315ebd231e6474fdf83))
* persist claude-cli credentials via /data volume ([78e78ef](https://github.com/zackpollard/discord-rpg-summariser/commit/78e78eff924a5ebb3f9a0484551f23da685f930c))

## [0.1.3](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.2...v0.1.3) (2026-03-23)


### Bug Fixes

* chain Docker build into release workflow ([210bd33](https://github.com/zackpollard/discord-rpg-summariser/commit/210bd33c4b1f2d36818dfddb5b170bb6eca1194c))

## [0.1.2](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.1...v0.1.2) (2026-03-23)


### Bug Fixes

* trigger Docker build on release event instead of tag push ([e7740dc](https://github.com/zackpollard/discord-rpg-summariser/commit/e7740dc6a5f0d3128657760eb7c2bbb98a46b7ad))

## [0.1.1](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.0...v0.1.1) (2026-03-23)


### Features

* add campaign stats dashboard with Chart.js visualizations ([b34b96b](https://github.com/zackpollard/discord-rpg-summariser/commit/b34b96be4d911236d0bcbec6ed4735c103a78995))
* add campaigns and knowledge base with entity extraction ([3c8b404](https://github.com/zackpollard/discord-rpg-summariser/commit/3c8b404b19769ae3492d17dd5315c0c4a9063257))
* add CI workflow with lint, format, build, and test checks ([dccf13a](https://github.com/zackpollard/discord-rpg-summariser/commit/dccf13a185f8307dae12b5538eb3c42c3bcdfd3d))
* add dev tooling with Docker Compose and Makefile ([fc217e7](https://github.com/zackpollard/discord-rpg-summariser/commit/fc217e7c7452a3dcb8bcc8fbb81db76d44dc61c4))
* add Discord bot with slash commands and recording pipeline ([875bc8b](https://github.com/zackpollard/discord-rpg-summariser/commit/875bc8b534258034c49a1b35d86415abfc99120e))
* add Discord OAuth2 authentication for web panel ([8ad14e1](https://github.com/zackpollard/discord-rpg-summariser/commit/8ad14e150bb49875e18077025b75afb2da1dbf63))
* add Dungeon Master role to campaigns ([bd3ca3b](https://github.com/zackpollard/discord-rpg-summariser/commit/bd3ca3bdbead9bc269b74dba29aae3d427966346))
* add e2e tests for TTS-to-transcription pipeline ([c8ac084](https://github.com/zackpollard/discord-rpg-summariser/commit/c8ac08488eb8824579e1904955acf158825a8e1b))
* add embedding storage with pgvector similarity search ([baddb6e](https://github.com/zackpollard/discord-rpg-summariser/commit/baddb6ee7e69e37e60cb51461cc650bb00e0b6e4))
* add entity timeline visualization with swimlane chart ([f6cd60b](https://github.com/zackpollard/discord-rpg-summariser/commit/f6cd60bf612f32dff69ca9825af97f112807e63b))
* add last:N option to campaign recap for recent session summaries ([f37b9d2](https://github.com/zackpollard/discord-rpg-summariser/commit/f37b9d263ec117a91574363d024ddfd3883aefbf))
* add live transcription with SSE streaming to web panel ([71c9529](https://github.com/zackpollard/discord-rpg-summariser/commit/71c95292aa060aac1e03be270162327abe4ddf72))
* add location hierarchy for place entities ([a33d466](https://github.com/zackpollard/discord-rpg-summariser/commit/a33d4663325cef321443a97a2b8de092e05f2c1b))
* add main entry point wiring all components ([a1a12d5](https://github.com/zackpollard/discord-rpg-summariser/commit/a1a12d53b0076a906ac20e0e78b05a6181eb5975))
* add NVIDIA Parakeet TDT 0.6B v3 transcription engine ([525e063](https://github.com/zackpollard/discord-rpg-summariser/commit/525e0632bda27d25b5a156fff64d270b121d6239))
* add pgvector infrastructure and embedding client ([58a9c27](https://github.com/zackpollard/discord-rpg-summariser/commit/58a9c273b301b6963265cdc54e6b5c3def72021b))
* add project skeleton with config, migrations, and storage layer ([362edb9](https://github.com/zackpollard/discord-rpg-summariser/commit/362edb99c51ccac435629b6cb4223d97f895632b))
* add quest tracker, campaign timeline, lore Q&A, and story recap ([fa7b731](https://github.com/zackpollard/discord-rpg-summariser/commit/fa7b731aba5ce40aa856e742c2b25c5788c29066))
* add README, Docker build, and CI/CD pipeline ([b6e0ccb](https://github.com/zackpollard/discord-rpg-summariser/commit/b6e0ccb17456b01b75b50f1caa7ceaf96063a374))
* add REST API server with session, transcript, and character endpoints ([aae23cc](https://github.com/zackpollard/discord-rpg-summariser/commit/aae23cc13151af0333f3ac1d0534bb88d62542a6))
* add session reprocessing API and UI ([e053799](https://github.com/zackpollard/discord-rpg-summariser/commit/e0537995f37dec52363e4a14dfa074ed2f100a39))
* add shared mic support to live transcription ([4607958](https://github.com/zackpollard/discord-rpg-summariser/commit/4607958bed87f11a0a9c40509e88858cccf5225c))
* add Storm King's Thunder seed data for testing ([3a797df](https://github.com/zackpollard/discord-rpg-summariser/commit/3a797df11fad72d409680438fd70ec64bc404360))
* add Svelte frontend with dashboard, sessions, and characters ([48761a0](https://github.com/zackpollard/discord-rpg-summariser/commit/48761a0e0b1a0936db600a19262cb90cb414b83e))
* add transcription and summarisation packages ([67f5da9](https://github.com/zackpollard/discord-rpg-summariser/commit/67f5da9467da01d77d5481933bdd97b80aa832c3))
* add voice recording and audio resampling ([f2a9fba](https://github.com/zackpollard/discord-rpg-summariser/commit/f2a9fba251b6eec3888c6f93e2a342bb0a94081a))
* audio playback with transcript synchronization ([9551eb9](https://github.com/zackpollard/discord-rpg-summariser/commit/9551eb9625a26f4ec06653dacff160f32d080fa4))
* cache Discord guild members in PostgreSQL ([bef5a89](https://github.com/zackpollard/discord-rpg-summariser/commit/bef5a897615cd9a0dee5fc92b897be71779c9961))
* combat detection and analysis from session transcripts ([0825e47](https://github.com/zackpollard/discord-rpg-summariser/commit/0825e47ab1aa1d2efc5b1bc8aa7635360c52ebd0))
* comprehensive test suite — 113 Go tests + 50 web tests ([0e1c3f5](https://github.com/zackpollard/discord-rpg-summariser/commit/0e1c3f59d550aa1960a451c08877c85c272ba2f2))
* cross-session entity references linking entities to transcript segments ([ad3e49e](https://github.com/zackpollard/discord-rpg-summariser/commit/ad3e49edbe033c814a1d932afedfe0ece17f57f5))
* D&D-style PDF campaign book generator ([dfa7cd4](https://github.com/zackpollard/discord-rpg-summariser/commit/dfa7cd4b1fcb3dfcfec01f3e4c5f87bf04d46a14))
* entity merging to combine duplicate lore entries ([9be56fd](https://github.com/zackpollard/discord-rpg-summariser/commit/9be56fd8ccea39156e9fa002d66462448753cdc9))
* fix audio dir creation, add live voice activity panel ([16f3fb8](https://github.com/zackpollard/discord-rpg-summariser/commit/16f3fb885f81a3bb8c53348e4cce265abaa032a6))
* full-text transcript search across sessions ([1295caa](https://github.com/zackpollard/discord-rpg-summariser/commit/1295caa5225bd27213e175d78670d33b83bd8f08))
* generate embeddings during session pipeline ([510b323](https://github.com/zackpollard/discord-rpg-summariser/commit/510b3235deb81eee08110f3472c0b5931c58cce4))
* implement DAVE E2EE frame decryption for voice receive ([4527ae4](https://github.com/zackpollard/discord-rpg-summariser/commit/4527ae4dae3d744a826d0c6ec70fd7c26e835a2e))
* interactive relationship graph visualization on lore page ([575f83f](https://github.com/zackpollard/discord-rpg-summariser/commit/575f83f39e4a40847c5678112e7aaa3821bacc64))
* log lost packets with sequence number and last bytes ([8dc0ce6](https://github.com/zackpollard/discord-rpg-summariser/commit/8dc0ce66949967e7cd072cf349601515aa03319f))
* RAG-powered lore Q&A with semantic search ([7dec7f0](https://github.com/zackpollard/discord-rpg-summariser/commit/7dec7f05d41e045dadcd355d0f34b7838884af3b))
* redesign shared mic, add voice enrollment, fix NPC extraction ([a6c0512](https://github.com/zackpollard/discord-rpg-summariser/commit/a6c05121b7232bdcb4d3c653f8c88ff6a3d0fe0b))
* show Discord usernames instead of IDs throughout the UI ([d29b526](https://github.com/zackpollard/discord-rpg-summariser/commit/d29b526a406413d86e8d2d150ed0e242b777397c))
* silence-aware chunked transcription via streaming resampler ([dbcfbd0](https://github.com/zackpollard/discord-rpg-summariser/commit/dbcfbd06e78edd6bcb860245390df3c40fa7eef4))
* sliding window live transcription with partial segment correction ([4c67dd4](https://github.com/zackpollard/discord-rpg-summariser/commit/4c67dd42ab9e41df1e0313e8453ca1842369b36a))
* speaker diarization for shared microphone support ([51396ee](https://github.com/zackpollard/discord-rpg-summariser/commit/51396ee3369184683f7ce2819c28a27a1610c640))
* switch to in-process whisper.cpp with auto model download ([6aabace](https://github.com/zackpollard/discord-rpg-summariser/commit/6aabace08eaad2dade4a946d07cf19f1f6f17ad9))
* Telegram integration for capturing DM messages during sessions ([80e5a5f](https://github.com/zackpollard/discord-rpg-summariser/commit/80e5a5f3312185825e1b49e62ebf577e32255062))
* track NPC dead/alive status with cause of death ([2da5c6b](https://github.com/zackpollard/discord-rpg-summariser/commit/2da5c6bd9695d68f61f37a8f5ad8cc6785f0b6c5))
* track player characters as first-class PC entities ([b84d528](https://github.com/zackpollard/discord-rpg-summariser/commit/b84d5283845e73f9dc689495bdbe4b5b778c82f7))


### Bug Fixes

* accumulate transcript segments instead of replacing them ([95c0f01](https://github.com/zackpollard/discord-rpg-summariser/commit/95c0f01f1c9308493d595e10c9ef92adc3e6fb1c))
* align frontend API URLs with backend route definitions ([67d8b1e](https://github.com/zackpollard/discord-rpg-summariser/commit/67d8b1ea08922a19cea2e00732bb08322a3f2668))
* cache Discord username lookups per request ([5b02d6f](https://github.com/zackpollard/discord-rpg-summariser/commit/5b02d6f0c96304db79d04ce5ee31b81983c4ba83))
* CI — add campaign_id to Session type, skip docker in integration tests ([aaf3ca2](https://github.com/zackpollard/discord-rpg-summariser/commit/aaf3ca2dbc229682f1c2768d7aa98e5ea4eab142))
* CI failures — missing reprocess.go, lore_search vet, svelte sync ([8240c46](https://github.com/zackpollard/discord-rpg-summariser/commit/8240c4666bf42073bec5ff3da9ec56d5ab8b54f1))
* clean up stale sessions on startup and reset state after stop ([5aab065](https://github.com/zackpollard/discord-rpg-summariser/commit/5aab065a750b153729ad8025c4b0d7886269ac97))
* configure vite proxy for SSE streaming (no buffering) ([e8baed9](https://github.com/zackpollard/discord-rpg-summariser/commit/e8baed9acd8c11fd25c1020c2d1db58a77a7d10e))
* copy opus data out of shared recv buffer to prevent race ([102901c](https://github.com/zackpollard/discord-rpg-summariser/commit/102901c814651946816471a3d9ad78e4c6e2eb1b))
* correct stubSummariser signature to match Summariser interface ([f530c31](https://github.com/zackpollard/discord-rpg-summariser/commit/f530c31237e85f84f2f2296abefd74748f8d7db4))
* decode as mono to match Discord's actual opus stream ([3b01e7d](https://github.com/zackpollard/discord-rpg-summariser/commit/3b01e7d1672b6b2bcecc4532420dc3ad9769e2f4))
* eliminate audio artifacts from corrupted decoder state ([8338dda](https://github.com/zackpollard/discord-rpg-summariser/commit/8338ddadb8a835dd392d8c2639071f90255d4758))
* fetch guild members from API instead of state cache ([d25c7bc](https://github.com/zackpollard/discord-rpg-summariser/commit/d25c7bcfff1a07d36cfeab598ba581afb236d3f1))
* Go formatting and Svelte type errors failing CI ([4d588e6](https://github.com/zackpollard/discord-rpg-summariser/commit/4d588e6465d90185711ab28f1b645bb1b4676967))
* handle encrypted silence frames and raw opus DTX correctly ([36162de](https://github.com/zackpollard/discord-rpg-summariser/commit/36162de6b389ca94a67791193a67ccae9524f944))
* highlight scrolled-to transcript segment with gold border ([73b9cd8](https://github.com/zackpollard/discord-rpg-summariser/commit/73b9cd8fb834fd838a58a2cfcdb07f4beca3934b))
* improve transcript search UX ([9ba5fc5](https://github.com/zackpollard/discord-rpg-summariser/commit/9ba5fc5d09e3157fd793af5fd28a1e833486a0ff))
* include all columns in ListCampaigns query ([602017f](https://github.com/zackpollard/discord-rpg-summariser/commit/602017fe225cc24ac1d4509f85b958afb703594b))
* keyword-based lore search and timeline UTF-8 handling ([6c4ad91](https://github.com/zackpollard/discord-rpg-summariser/commit/6c4ad91a1d7fab0b54663d3f411bff905c784a71))
* patch discordgo fork for bot passthrough DAVE (no E2EE on recv) ([0b00861](https://github.com/zackpollard/discord-rpg-summariser/commit/0b008613446c1331f6d5ceb0055cfb3591140348))
* PDF campaign book generator encoding, layout, and page output ([8ac25ec](https://github.com/zackpollard/discord-rpg-summariser/commit/8ac25ec4c6b0a1119516bd3ba0feb09ef62613e6))
* poll for live transcript worker and add diagnostic logging ([d1d94bf](https://github.com/zackpollard/discord-rpg-summariser/commit/d1d94bfe2b6927eedbb6912f73d61526d8d1474a))
* remove stale DAVE frame stripping, expand diagnostic logging ([fbef9c5](https://github.com/zackpollard/discord-rpg-summariser/commit/fbef9c5f03198fd4d0eb118c84f5dc8165253557))
* remove unused storage import in transcripts.go ([7fb655e](https://github.com/zackpollard/discord-rpg-summariser/commit/7fb655eee291e82e15fa759f319b4f4740dfb9da))
* remove unused time import in liveworker ([8794af5](https://github.com/zackpollard/discord-rpg-summariser/commit/8794af5ee7808fa9dc218066d0515a94a2c38cb1))
* replace SetOffset with manual timestamp shifting ([9f5549d](https://github.com/zackpollard/discord-rpg-summariser/commit/9f5549de4a78961d3aec90a599e05257d8f02b2d))
* revert DAVE passthrough, add frame stripping and stereo decode ([ee872d5](https://github.com/zackpollard/discord-rpg-summariser/commit/ee872d538891457c9bb7e829a82f7e6b4592ec44))
* run integration test packages sequentially to avoid migration race ([f364b2f](https://github.com/zackpollard/discord-rpg-summariser/commit/f364b2f77b8425b69b3627aa1130ac0a52b9bed7))
* scan for DAVE trailer to handle misaligned extension stripping ([b486471](https://github.com/zackpollard/discord-rpg-summariser/commit/b486471317527308a16e8a46683d31bf0410e44b))
* scroll to transcript segment on hash navigation ([e61a885](https://github.com/zackpollard/discord-rpg-summariser/commit/e61a88557372c8bc983d9088ccaa80962111cf12))
* set CGO flags for whisper.cpp in integration test job ([16e5fc4](https://github.com/zackpollard/discord-rpg-summariser/commit/16e5fc466098cf4f65d65681e4c349b92f7073cc))
* set LD_LIBRARY_PATH for whisper.cpp in integration tests ([76953f2](https://github.com/zackpollard/discord-rpg-summariser/commit/76953f29c253b37fa0a40bc262a860cc7e9cb43d))
* stop storing 'User-{id}' fallback as character_name ([add9ca5](https://github.com/zackpollard/discord-rpg-summariser/commit/add9ca5f9a2f507b49cc394364463ba1a1867d6a))
* switch to discordgo fork with DAVE E2EE voice support ([fc5738d](https://github.com/zackpollard/discord-rpg-summariser/commit/fc5738de67a74cdc57d14db3af4d5c91d1e48517))
* use AES-CTR for DAVE decryption (Go GCM requires tag &gt;= 12) ([05e70cc](https://github.com/zackpollard/discord-rpg-summariser/commit/05e70cc99578e32c8258bb5719818eed09eb200d))
* validate DAVE frame supplemental size to avoid false positives ([b9da4d6](https://github.com/zackpollard/discord-rpg-summariser/commit/b9da4d60abd196e72edbb83ffec5d4b88256bb2e))


### Code Refactoring

* add parsePathID and parsePagination helpers to reduce API boilerplate ([06cbe45](https://github.com/zackpollard/discord-rpg-summariser/commit/06cbe45f81a38f9ebabcfbda8685ab603cd30bf8))
* clean up DAVE decryption, skip pre-transition packets ([6a26b1e](https://github.com/zackpollard/discord-rpg-summariser/commit/6a26b1e65618fe3882842712d4c6c0cbb465b13d))
* clean up voice recording code ([15bd1d6](https://github.com/zackpollard/discord-rpg-summariser/commit/15bd1d6c3d3bbe06d02a0d78a54da9c43b46126b))
* deduplicate LLM extraction methods with runPrompt helper ([8780ba6](https://github.com/zackpollard/discord-rpg-summariser/commit/8780ba6d9cea5d9438f95b98f048eac589973172))
* drop character_name column from transcript_segments ([03cc196](https://github.com/zackpollard/discord-rpg-summariser/commit/03cc196cfc6ddb4b99c7462a916fa57b7924e226))
* extract displayNameResolver and clean up test helpers ([afb9671](https://github.com/zackpollard/discord-rpg-summariser/commit/afb96718226f11fb60602d9af233bfd6dd85da04))
* resolve character names at display time, not storage time ([d436610](https://github.com/zackpollard/discord-rpg-summariser/commit/d4366104eae088537c8edc8141430245611c5e7a))
* restructure UI around campaigns as the entrypoint ([4a4df3f](https://github.com/zackpollard/discord-rpg-summariser/commit/4a4df3f0c4cbf381b43a385e5c8eda0011c28af8))
* split handlers.go into pipeline.go and enrollment.go ([eb4eb8d](https://github.com/zackpollard/discord-rpg-summariser/commit/eb4eb8d5102f333b844ef9c1598e9482492a848f))


### Documentation

* add demo video walkthrough and update screenshots ([9752227](https://github.com/zackpollard/discord-rpg-summariser/commit/9752227ad5d40acb0bd758a2d5b8c4e8f2acaed9))
* add screenshots and update README for new features ([4c2e63d](https://github.com/zackpollard/discord-rpg-summariser/commit/4c2e63d198b91c298630b6ffe1e495f55ca5cdda))
* link demo video directly from repo ([26f239e](https://github.com/zackpollard/discord-rpg-summariser/commit/26f239ec60d0f3dd3646a5fb80159855c627fd0b))
* re-record demo video at 1280x720, ~47s walkthrough ([5843b00](https://github.com/zackpollard/discord-rpg-summariser/commit/5843b00be678206b7bb3c738ada11ebeec88888f))
* re-record demo with smooth eased scrolling at 30fps ([ca16390](https://github.com/zackpollard/discord-rpg-summariser/commit/ca163903878b10178f25ce656d5633f2b41babc0))
* update README, config, and Docker setup for recent features ([a6617d2](https://github.com/zackpollard/discord-rpg-summariser/commit/a6617d23378a721c4983a05a0a6976da3d7f8c3a))


### Miscellaneous

* update demo video ([f321d78](https://github.com/zackpollard/discord-rpg-summariser/commit/f321d780c8c842bf5418b541a0ac27aff676caf3))


### CI/CD

* add release-please for automated versioning and changelog ([6ee5f17](https://github.com/zackpollard/discord-rpg-summariser/commit/6ee5f1720ca274676e38498446627e6e87e862d1))
