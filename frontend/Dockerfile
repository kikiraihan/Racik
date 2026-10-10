FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# Target proxy /api & /uploads ditanam saat build (lihat nuxt.config.ts)
ARG NUXT_API_TARGET=http://backend:8080
ENV NUXT_API_TARGET=$NUXT_API_TARGET
RUN npm run build

FROM node:22-alpine
WORKDIR /app
COPY --from=build /app/.output ./.output
ENV HOST=0.0.0.0 PORT=3000
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
