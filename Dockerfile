FROM golang:1.23.1 AS base

WORKDIR /app

FROM node:20 AS build
WORKDIR /src
COPY . .

RUN npm install tailwindcss @tailwindcss/cli
RUN npx tailwindcss -i ./static/main.css -o ./static/tailwind.css

FROM base AS final 
WORKDIR /app

COPY go.mod go.sum . ./
COPY --from=build ./src .

RUN CGO_ENABLED=0 GOOS=linux go build -o /front_desk

CMD ["/front_desk"]