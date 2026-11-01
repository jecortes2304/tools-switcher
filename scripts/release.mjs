import { readFile, appendFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { pathToFileURL } from "node:url";

export const conventionalTitle =
  /^(feat|fix|perf|refactor|docs|test|build|ci|chore|style|revert)(\([a-zA-Z0-9._/-]+\))?!?: \S.+$/;
export function validTitle(pr) {
  if (pr.base.ref === "main") return pr.head.ref === "develop";
  if (pr.head.ref === "main" && pr.base.ref === "develop") return true;
  return conventionalTitle.test(pr.title);
}
export function consistentVersion(version, manifest, pkg, lock) {
  return (
    /^\d+\.\d+\.\d+$/.test(version) &&
    [manifest["."], pkg.version, lock.version, lock.packages[""].version].every(
      (value) => value === version,
    )
  );
}
export function releasePlan(version, latest, ancestor) {
  const compare = version
    .split(".")
    .map(Number)
    .reduce(
      (result, value, index) =>
        result || Math.sign(value - Number(latest.split(".")[index])),
      0,
    );
  if (compare < 0)
    throw new Error(
      "develop está por detrás de la última release; sincroniza main antes de continuar.",
    );
  if (version !== latest)
    return {
      ready: false,
      message: `La versión ${version} ya está preparada. Promueve develop a main para publicarla.`,
    };
  if (!ancestor)
    throw new Error(
      "Sincroniza main en develop antes de preparar la siguiente versión; falta el último tag en su historial.",
    );
  return { ready: true, message: "Preparando el PR de la próxima versión." };
}
async function output(name, value) {
  if (process.env.GITHUB_OUTPUT)
    await appendFile(process.env.GITHUB_OUTPUT, `${name}=${value}\n`);
}
async function github(path, method = "GET", body) {
  const response = await fetch(
    `https://api.github.com/repos/${process.env.GITHUB_REPOSITORY}${path}`,
    {
      method,
      headers: {
        Authorization: `Bearer ${process.env.GH_TOKEN}`,
        Accept: "application/vnd.github+json",
        "Content-Type": "application/json",
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    },
  );
  if (!response.ok)
    throw new Error(`GitHub ${method} ${path}: HTTP ${response.status}`);
  return response.status === 204 ? null : response.json();
}
async function configuration() {
  const [version, manifest, pkg, lock] = await Promise.all(
    [
      "VERSION",
      ".release-please-manifest.json",
      "frontend/package.json",
      "frontend/package-lock.json",
    ].map((path) => readFile(path, "utf8")),
  );
  const value = version.trim();
  if (
    !consistentVersion(
      value,
      JSON.parse(manifest),
      JSON.parse(pkg),
      JSON.parse(lock),
    )
  )
    throw new Error(
      "VERSION, manifiesto y frontend deben coincidir. Integra el PR generado por Release Please.",
    );
  return value;
}
async function markPublished(version) {
  const prs = await github(
    "/pulls?state=closed&base=develop&sort=updated&direction=desc&per_page=100",
  );
  for (const pr of prs) {
    if (
      !pr.merged_at ||
      !pr.head.ref.startsWith("release-please--") ||
      !pr.labels.some((label) => label.name === "autorelease: pending")
    )
      continue;
    const file = await github(`/contents/VERSION?ref=${pr.merge_commit_sha}`);
    if (Buffer.from(file.content, "base64").toString("utf8").trim() !== version)
      continue;
    const labels = await github("/labels?per_page=100");
    if (!labels.some((label) => label.name === "autorelease: tagged"))
      await github("/labels", "POST", {
        name: "autorelease: tagged",
        color: "0e8a16",
      });
    await github(
      `/issues/${pr.number}/labels/${encodeURIComponent("autorelease: pending")}`,
      "DELETE",
    );
    await github(`/issues/${pr.number}/labels`, "POST", {
      labels: ["autorelease: tagged"],
    });
  }
}
async function dispatch(workflow, ref) {
  await github(`/actions/workflows/${workflow}/dispatches`, "POST", { ref });
}
async function syncPublished() {
  const version = process.env.RELEASE_VERSION;
  const sha = process.env.GITHUB_SHA;
  await markPublished(version);
  const compare = await github(`/compare/develop...${sha}`);
  if (compare.status === "identical") return;
  if (compare.status === "ahead") {
    let advanced = false;
    try {
      await github("/git/refs/heads/develop", "PATCH", { sha, force: false });
      advanced = true;
    } catch (error) {
      console.log(`Sincronización automática pendiente: ${error.message}`);
    }
    if (advanced) {
      await dispatch("release-please.yml", "develop");
      return;
    }
  }
  const prs = await github(
    "/pulls?state=open&base=develop&head=" +
      encodeURIComponent(process.env.GITHUB_REPOSITORY.split("/")[0] + ":main"),
  );
  if (!prs.length) {
    const pr = await github("/pulls", "POST", {
      title: `chore: sincronizar main después de v${version}`,
      head: "main",
      base: "develop",
      body: "La release está publicada. Integra este PR antes de preparar la siguiente versión. Hay cambios nuevos en develop y no se ha modificado su historial.",
    });
    console.log(`Sincronización pendiente: ${pr.html_url}`);
    await dispatch("features.yml", "main");
  }
}
export async function main(command) {
  if (command === "title") {
    const event = JSON.parse(
      await readFile(process.env.GITHUB_EVENT_PATH, "utf8"),
    );
    if (event.pull_request && !validTitle(event.pull_request))
      throw new Error(
        "Usa un título convencional: feat: ..., fix: ..., docs: ...; hacia main solo se admite develop.",
      );
  } else if (command === "version") {
    console.log(`Versión coherente: ${await configuration()}`);
  } else if (command === "publish") {
    const version = await configuration();
    const prs = await github(
      "/pulls?state=closed&base=develop&sort=updated&direction=desc&per_page=100",
    );
    let candidate;
    for (const pr of prs) {
      if (!pr.merged_at || !pr.head.ref.startsWith("release-please--"))
        continue;
      const file = await github(`/contents/VERSION?ref=${pr.merge_commit_sha}`);
      if (
        Buffer.from(file.content, "base64").toString("utf8").trim() === version
      ) {
        candidate = pr;
        break;
      }
    }
    if (!candidate)
      throw new Error(
        "Falta el PR de release preparado por el bot. Fusiónalo en develop antes de promover a main.",
      );
    const comparison = await github(
      `/compare/${candidate.merge_commit_sha}...${process.env.GITHUB_SHA}`,
    );
    if (comparison.files.length !== 0)
      throw new Error(
        "Hay cambios posteriores al PR de release. Publica el candidato revisado antes de integrar más features.",
      );
  } else if (command === "plan") {
    const version = await configuration();
    const latest = await github("/releases/latest");
    const latestVersion = latest.tag_name.replace(/^v/, "");
    let ancestor = true;
    try {
      execFileSync(
        "git",
        ["merge-base", "--is-ancestor", latest.tag_name, "HEAD"],
        { stdio: "ignore" },
      );
    } catch {
      ancestor = false;
    }
    const plan = releasePlan(version, latestVersion, ancestor);
    if (version === latestVersion) await markPublished(version);
    console.log(plan.message);
    await output("ready", plan.ready);
  } else if (command === "validate-prs") {
    for (const pr of JSON.parse(process.env.RELEASE_PRS || "[]")) {
      if (!pr.headBranchName?.startsWith("release-please--"))
        throw new Error("Rama de release inesperada");
      await dispatch("features.yml", pr.headBranchName);
    }
  } else if (command === "sync") await syncPublished();
  else throw new Error("Comando desconocido");
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)
  main(process.argv[2]).catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
