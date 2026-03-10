# Algorhythm full-stack — оркестрация инфраструктуры

.PHONY: up down logs ps env

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

env:
	@if [ ! -f .env ]; then cp .env.example .env && echo "Created .env from .env.example"; fi
