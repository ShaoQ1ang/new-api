FROM node:20-alpine

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG ALL_PROXY
ARG http_proxy
ARG https_proxy
ARG all_proxy

ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTPS_PROXY}
ENV ALL_PROXY=${ALL_PROXY}
ENV http_proxy=${http_proxy}
ENV https_proxy=${https_proxy}
ENV all_proxy=${all_proxy}

WORKDIR /app

COPY package.json ./
RUN npm install

COPY . .

EXPOSE 3001

CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "3001"]
