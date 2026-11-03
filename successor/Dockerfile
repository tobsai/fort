# Fort cloud hub (Spec 063). A plain image with no host-specific SDK: any VM or container host
# that gives it a port, a persistent volume at /data and TLS in front can run it.
FROM node:26.5.0-bookworm-slim
# Spec 063, revision 2026-11-03: the hub runs ChatGPT Agents itself with the same Codex release the Mac pins (0.163.0),
# fetched from npm and checked against this digest before anything is unpacked. ca-certificates lets Codex reach ChatGPT;
# setpriv (util-linux, already in the base image) ends Codex if its executor dies.
ARG CODEX_VERSION=0.163.0
ARG CODEX_SHA256=30efc97738b975940d78b59a9bea115fa34040b732807d54ec35ae6b7528de94
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* \
 && test -x /usr/bin/setpriv \
 && node -e "const [v,d]=process.argv.slice(1);fetch('https://registry.npmjs.org/@openai/codex/-/codex-'+v+'-linux-x64.tgz').then(async r=>{if(!r.ok)throw new Error('download '+r.status);const b=Buffer.from(await r.arrayBuffer());const h=require('crypto').createHash('sha256').update(b).digest('hex');if(h!==d)throw new Error('Codex digest mismatch: '+h);require('fs').writeFileSync('/tmp/codex.tgz',b)}).catch(e=>{console.error(e.message);process.exit(1)})" "$CODEX_VERSION" "$CODEX_SHA256" \
 && mkdir -p /tmp/codex /opt/fort/codex && tar -xzf /tmp/codex.tgz -C /tmp/codex \
 && install -m 0755 /tmp/codex/package/vendor/x86_64-unknown-linux-musl/bin/codex /opt/fort/codex/codex \
 && rm -rf /tmp/codex /tmp/codex.tgz \
 && /opt/fort/codex/codex --version
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
