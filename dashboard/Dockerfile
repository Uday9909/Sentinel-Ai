# syntax=docker/dockerfile:1
FROM node:22-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# ----------------------------------------------------------
FROM node:22-alpine AS deps

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev

# ----------------------------------------------------------
FROM node:22-alpine

WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY server.js ./
COPY --from=builder /app/dist ./dist

ENV NODE_ENV=production
EXPOSE 3001
CMD ["node", "server.js"]
