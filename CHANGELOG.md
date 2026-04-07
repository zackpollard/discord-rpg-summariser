# Changelog

## [0.1.13](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.12...v0.1.13) (2026-04-07)


### Features

* show transcript annotations in the UI ([c0f2955](https://github.com/zackpollard/discord-rpg-summariser/commit/c0f29554ad572f4dfad91fa24bd0efa202bf037c))


### Bug Fixes

* add annotation and title/quotes stages to reprocess pipeline ([6ee8b6a](https://github.com/zackpollard/discord-rpg-summariser/commit/6ee8b6a8e4091ceaaa62dd41f9ea06444622ee80))
* allow empty quotes array when nothing is quote-worthy ([0dfec4c](https://github.com/zackpollard/discord-rpg-summariser/commit/0dfec4c2108438da35cbd87ccc4144e8c15ea403))
* auth bypass for local dev and hidden nav scrollbar ([3682a67](https://github.com/zackpollard/discord-rpg-summariser/commit/3682a67b3dffee618bd16c99c0cce7795a28ad36))
* keep table talk in transcript marked as [TABLE TALK] ([717abce](https://github.com/zackpollard/discord-rpg-summariser/commit/717abce12c2c09c5b613fd23d2c7fb1985ebf908))
* UI polish from code review ([a8583c6](https://github.com/zackpollard/discord-rpg-summariser/commit/a8583c6dffed4dead09efa93a7ff5b403a020644))

## [0.1.12](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.11...v0.1.12) (2026-04-07)


### Features

* add /campaign play-recap to play TTS recap in voice channel ([3c62f79](https://github.com/zackpollard/discord-rpg-summariser/commit/3c62f7978b464fa29f5f08580b126c6354d0192f))
* add custom voice profiles for TTS recap ([cb68c68](https://github.com/zackpollard/discord-rpg-summariser/commit/cb68c682aa52a030940d70f68f009be0e7dc9414))
* add recap styles, previously-on, character summaries, combat analysis, clip name suggestions ([65403ac](https://github.com/zackpollard/discord-rpg-summariser/commit/65403acf56c3271a3c44b2ccd9e2e1f4625a3277))
* add session title generation and memorable quote extraction ([90243f1](https://github.com/zackpollard/discord-rpg-summariser/commit/90243f11efa23db4bd7987d6925fdd74cc89f19f))
* add soundboard with clip creation from transcripts ([4a38d69](https://github.com/zackpollard/discord-rpg-summariser/commit/4a38d6914a4ee8a0eab159a39599124195405c6e))
* add transcript annotation pipeline stage ([586392b](https://github.com/zackpollard/discord-rpg-summariser/commit/586392b4ff1436f089932de26c47bdbf189b4ff5))
* soundboard, waveform editor, DM label fix, LLM parsing fix ([06dfeab](https://github.com/zackpollard/discord-rpg-summariser/commit/06dfeab6cb7a6ca6d21da24c8fcf49f7a9cf5e45))


### Bug Fixes

* re-derive DAVE keys when new users join voice channel ([48c2a4e](https://github.com/zackpollard/discord-rpg-summariser/commit/48c2a4eba9c3e8a6155612ead68cf2ec42388c47))


### Code Refactoring

* separate recap audio generation from playback ([2ccdf32](https://github.com/zackpollard/discord-rpg-summariser/commit/2ccdf32c4d1b42abef484f1c8f7a00e149b8b6ee))

## [0.1.11](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.10...v0.1.11) (2026-04-06)


### Features

* add intra-file progress during transcription ([5c06f19](https://github.com/zackpollard/discord-rpg-summariser/commit/5c06f19b49f1450370cf87ae27e047cb97a1f06b))


### Bug Fixes

* add python TTS venv to docker build ([c1c0e09](https://github.com/zackpollard/discord-rpg-summariser/commit/c1c0e09b0038a1a12d66b8a6d6ca809072359d00))
* auto-generate bpe.vocab for parakeet hot words ([4b21eae](https://github.com/zackpollard/discord-rpg-summariser/commit/4b21eaeab1cb63c4dfc0e31b40a5d8dafbba10c6))
* improve reprocess progress tracking ([70a30b5](https://github.com/zackpollard/discord-rpg-summariser/commit/70a30b54881eedbcff8e1d96c06834aeb3275a08))
* reduce peak memory usage during transcription ([b69c3c5](https://github.com/zackpollard/discord-rpg-summariser/commit/b69c3c595dfeb625306bd51993769988abd9c4a7))
* use per-segment embeddings for shared mic speaker attribution ([bdd5586](https://github.com/zackpollard/discord-rpg-summariser/commit/bdd5586aaf86b89d288c1a6292a1de274b535bde))

## [0.1.10](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.9...v0.1.10) (2026-03-31)


### Features

* add pipeline progress, LLM debug logs, audio fixes, and TTS recap ([b38d818](https://github.com/zackpollard/discord-rpg-summariser/commit/b38d818d3cc5794f3a83f5cba49bca5dd1234c7e))

## [0.1.9](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.8...v0.1.9) (2026-03-24)


### Features

* add campaign settings page with game system field ([c9b0793](https://github.com/zackpollard/discord-rpg-summariser/commit/c9b07930dc606b11b47c881dce88f9a20ad5b79f))
* add session deletion and entity renaming ([2ce34cf](https://github.com/zackpollard/discord-rpg-summariser/commit/2ce34cff20cdcb76593438a3848ccae54bd36647))
* bias transcription toward campaign-specific vocabulary ([5d43208](https://github.com/zackpollard/discord-rpg-summariser/commit/5d4320894ecef4b54061fc2050f1834f9e0355c0))
* replace Ollama embeddings with in-process ONNX inference ([db5577e](https://github.com/zackpollard/discord-rpg-summariser/commit/db5577e62ab9675b8acdb5457021b21d05f0cc7c))
* use Claude Opus with high effort for summarisation ([4eff550](https://github.com/zackpollard/discord-rpg-summariser/commit/4eff550b70d8f53f9fca8449946ad7fa07eafa7c))


### Bug Fixes

* create /data dirs and claude symlink at runtime, not build time ([1917823](https://github.com/zackpollard/discord-rpg-summariser/commit/19178231e1658ad9fabae8d7996c73acd1cd3919))

## [0.1.8](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.7...v0.1.8) (2026-03-24)


### Bug Fixes

* include scripts directory in Docker image ([6f127f8](https://github.com/zackpollard/discord-rpg-summariser/commit/6f127f8b01695843be79100e3311c9bd03de4fad))
* remove 5-second silence cap to preserve full speech gaps ([239d4a2](https://github.com/zackpollard/discord-rpg-summariser/commit/239d4a2209abd790390cdf6e690b26f0dc3b4ebe))

## [0.1.7](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.6...v0.1.7) (2026-03-23)


### Features

* improve summarization prompt for DM NPC voice attribution ([4a4d41e](https://github.com/zackpollard/discord-rpg-summariser/commit/4a4d41ef7fd81280168d1e660bee8e1b2dfafe21))


### Bug Fixes

* adjust transcript timestamps for users who join late ([c803882](https://github.com/zackpollard/discord-rpg-summariser/commit/c8038824576d9c61c6adc097e81521ef6466ca5a))
* include stdout in claude CLI error messages ([3d56407](https://github.com/zackpollard/discord-rpg-summariser/commit/3d5640727bc80fb03a29eb56cf38865ffbfe603f))
* insert correct silence gap on user disconnect/reconnect ([674bfad](https://github.com/zackpollard/discord-rpg-summariser/commit/674bfad51f1877388c757407c2e3e6fac9952692))
* label DM as "DM" in transcript instead of character name ([fe23d7c](https://github.com/zackpollard/discord-rpg-summariser/commit/fe23d7caa9dcce53ee202fbd2984f4031b3ff5a0))
* make notification link URL configurable ([24ddd07](https://github.com/zackpollard/discord-rpg-summariser/commit/24ddd07112b0e2774a1015578934da44ac227915))
* persist join offsets to offsets.json for reprocessing ([37c5503](https://github.com/zackpollard/discord-rpg-summariser/commit/37c550334f7ff8e9e47a6aded0226bb1ad8c8689))
* write offsets.json on user join, handle reconnects ([885133c](https://github.com/zackpollard/discord-rpg-summariser/commit/885133c0b87a24a39248c3bc191303b9e0ab554b))


### Performance

* lazy-load transcription model to free ~22GB idle memory ([9c9938b](https://github.com/zackpollard/discord-rpg-summariser/commit/9c9938b7f8e8d24b595ec430daa630becc3f4b57))


### Miscellaneous

* add script to generate offsets.json from WAV file timestamps ([e870be9](https://github.com/zackpollard/discord-rpg-summariser/commit/e870be940ab29006ef2e3114bffaea1962802d69))

## [0.1.6](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.5...v0.1.6) (2026-03-23)


### Features

* log version on startup ([5a34435](https://github.com/zackpollard/discord-rpg-summariser/commit/5a344355b612f2b27f358dcd4097efae33e725f8))


### Bug Fixes

* pin whisper.cpp to v1.8.4 in CI and Makefile ([6d0138a](https://github.com/zackpollard/discord-rpg-summariser/commit/6d0138a79e027cf7ca466b81eb99b6f025af0c52))
* voice recording, live transcription, and member sync ([23c4749](https://github.com/zackpollard/discord-rpg-summariser/commit/23c47491afeabe5f6e0af2e919f10fe44b10985e))


### Miscellaneous

* add debug logging for voice connection and packet reception ([d62e492](https://github.com/zackpollard/discord-rpg-summariser/commit/d62e492d02f0e2b080a3b5354751ace35aebf250))

## [0.1.5](https://github.com/zackpollard/discord-rpg-summariser/compare/v0.1.4...v0.1.5) (2026-03-23)


### Bug Fixes

* add logging for silent failures in guild member sync ([a1527f0](https://github.com/zackpollard/discord-rpg-summariser/commit/a1527f070baa5aa970445090d5f8d66f9aa83c1d))
* create session audio directory before recording ([0a4e582](https://github.com/zackpollard/discord-rpg-summariser/commit/0a4e582162fcee29da5172444012aaf32ac978ba))
* pre-create pgvector extension via init script ([fa37cdf](https://github.com/zackpollard/discord-rpg-summariser/commit/fa37cdfec6179e19b832b9bfc9d9e306dec2b2b7))

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
