# Fort cloud hub (Spec 063). A plain image with no host-specific SDK: any VM or container host
# that gives it a port, a persistent volume at /data and TLS in front can run it.
FROM node:26.5.0-bookworm-slim
WORKDIR /app
COPY package.json ./
COPY contracts contracts
COPY modules modules
COPY runner runner
COPY service service
COPY host host
COPY web web
ENV NODE_ENV=production FORT_DATA=/data
EXPOSE 8080
CMD ["node", "host/cloud-hub.ts"]
