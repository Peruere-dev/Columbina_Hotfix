#!/usr/bin/env python3
import json, os, sys, hashlib, xxhash
from urllib.request import Request, urlopen
from concurrent.futures import ThreadPoolExecutor, as_completed

DATA_ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data")
MAX_WORKERS = 30
PLATFORM_MAP = {"Android": "Android", "Win": "StandaloneWindows64", "iOS": "iOS"}


def download(url, timeout=30):
    try:
        req = Request(url, headers={"User-Agent": "Mozilla/5.0"})
        with urlopen(req, timeout=timeout) as r:
            return r.read()
    except Exception:
        return None


def extract_branch(resource_url):
    return resource_url.rstrip("/").rsplit("/", 1)[-1] if resource_url else ""


def parse_md5_lines(text):
    entries = []
    for line in text.split("\n"):
        line = line.strip()
        if not line:
            continue
        try:
            entries.append(json.loads(line))
        except json.JSONDecodeError:
            pass
    return entries


def collect_tasks(cfg, area, plat, version, file_path):
    branch = extract_branch(cfg.get("ResourceUrl", "")) or cfg.get("ResVersionConfig", {}).get("Branch", "")
    rv = cfg.get("ResVersionConfig", {})
    res_ver = rv.get("Version", 0)
    res_suf = rv.get("VersionSuffix", "")
    label = f"{area}/{plat}/{version}"
    tasks = []

    raw = cfg.get("ClientDataMd5", "")
    if raw:
        try:
            obj = json.loads(raw)
        except json.JSONDecodeError:
            obj = None
        if obj and "remoteName" in obj:
            base = cfg.get("DataUrl", "")
            ver = cfg.get("ClientDataVersion")
            suf = cfg.get("ClientVersionSuffix")
            if base and ver is not None and suf:
                url = f"{base}/output_{ver}_{suf}/client/General/AssetBundles/{obj['remoteName']}"
                tasks.append((url, obj, label, area, plat, version, branch, res_ver, res_suf, file_path, "ClientDataMd5"))

    raw = cfg.get("ClientSilenceDataMd5", "")
    if raw:
        try:
            obj = json.loads(raw)
        except json.JSONDecodeError:
            obj = None
        if obj and "remoteName" in obj:
            base = cfg.get("DataUrl", "")
            ver = cfg.get("ClientSilenceDataVersion")
            suf = cfg.get("ClientSilenceVersionSuffix")
            if base and ver is not None and suf:
                url = f"{base}/output_{ver}_{suf}/client_silence/General/AssetBundles/{obj['remoteName']}"
                tasks.append((url, obj, label, area, plat, version, branch, res_ver, res_suf, file_path, "ClientSilenceDataMd5"))

    rv_cfg = cfg.get("ResVersionConfig", {})
    md5_text = rv_cfg.get("Md5", "")
    if md5_text:
        base = cfg.get("ResourceUrl", "")
        ver = rv_cfg.get("Version")
        suf = rv_cfg.get("VersionSuffix")
        if base and ver is not None and suf:
            cdn_plat = PLATFORM_MAP.get(plat, plat)
            for entry in parse_md5_lines(md5_text):
                rn = entry.get("remoteName")
                if rn:
                    url = f"{base}/output_{ver}_{suf}/client/{cdn_plat}/{rn}"
                    tasks.append((url, entry, label, area, plat, version, branch, res_ver, res_suf, file_path, "ResVersionConfig"))

    return tasks


def save_path(area, plat, version, branch, res_ver, res_suf, cdn_url):
    prefix = f"output_{res_ver}_{res_suf}"
    idx = cdn_url.find(prefix)
    subpath = cdn_url[idx + len(prefix) + 1:] if idx >= 0 else cdn_url.split("/")[-1]
    return os.path.join(DATA_ROOT, area, plat, version, branch, prefix, subpath)


def ensure_dir(path):
    d = os.path.dirname(path)
    if d and not os.path.exists(d):
        os.makedirs(d, exist_ok=True)


def process_one(task):
    url, entry, label, area, plat, version, branch, res_ver, res_suf, file_path, field_tag = task
    data = download(url)
    rn = entry.get("remoteName", "?")
    if data is None:
        return label, f"{rn} 404", None

    st = save_path(area, plat, version, branch, res_ver, res_suf, url)
    md5 = hashlib.md5(data).hexdigest()
    xxh = xxhash.xxh64(data).hexdigest()
    file_size = len(data)
    ensure_dir(st)
    with open(st, "wb") as f:
        f.write(data)

    expected_md5 = entry.get("md5", "")
    expected_size = entry.get("fileSize", 0)

    needs_fix = False
    if not expected_md5 or expected_size == 0:
        needs_fix = True
    elif md5 != expected_md5 or file_size != expected_size:
        needs_fix = True

    if needs_fix:
        fix = {
            "file_path": file_path,
            "field_tag": field_tag,
            "remote_name": rn,
            "md5": md5,
            "file_size": file_size,
            "hash": xxh,
        }
        reason = []
        if expected_md5 or expected_size:
            if expected_md5 and md5 != expected_md5:
                reason.append("md5")
            if expected_size and file_size != expected_size:
                reason.append("size")
        else:
            reason.append("empty")
        return label, f"{rn} {'/'.join(reason) if reason else 'empty'}", fix

    return label, None, None


def apply_fixes(fixes):
    if not fixes:
        return
    by_file = {}
    for fix in fixes:
        by_file.setdefault(fix["file_path"], []).append(fix)

    for file_path, file_fixes in by_file.items():
        with open(file_path, "r") as f:
            cfg = json.load(f)
        modified = False

        for fix in file_fixes:
            tag = fix["field_tag"]
            rn = fix["remote_name"]
            new_md5 = fix["md5"]
            new_size = fix["file_size"]
            new_hash = fix["hash"]

            if tag == "ClientDataMd5":
                raw = cfg.get("ClientDataMd5", "")
                try:
                    obj = json.loads(raw)
                except (json.JSONDecodeError, TypeError):
                    continue
                if obj.get("remoteName") == rn and (obj.get("md5") != new_md5 or obj.get("fileSize") != new_size):
                    obj["md5"] = new_md5
                    obj["fileSize"] = new_size
                    obj["hash"] = new_hash
                    cfg["ClientDataMd5"] = json.dumps(obj)
                    modified = True

            elif tag == "ClientSilenceDataMd5":
                raw = cfg.get("ClientSilenceDataMd5", "")
                try:
                    obj = json.loads(raw)
                except (json.JSONDecodeError, TypeError):
                    continue
                if obj.get("remoteName") == rn and (obj.get("md5") != new_md5 or obj.get("fileSize") != new_size):
                    obj["md5"] = new_md5
                    obj["fileSize"] = new_size
                    obj["hash"] = new_hash
                    cfg["ClientSilenceDataMd5"] = json.dumps(obj)
                    modified = True

            elif tag == "ResVersionConfig":
                rv_cfg = cfg.get("ResVersionConfig", {})
                md5_text = rv_cfg.get("Md5", "")
                lines = md5_text.split("\n")
                for i, line in enumerate(lines):
                    stripped = line.strip()
                    if not stripped:
                        continue
                    try:
                        entry = json.loads(stripped)
                    except json.JSONDecodeError:
                        continue
                    if entry.get("remoteName") == rn and (entry.get("md5") != new_md5 or entry.get("fileSize") != new_size):
                        entry["md5"] = new_md5
                        entry["fileSize"] = new_size
                        entry["hash"] = new_hash
                        lines[i] = json.dumps(entry)
                        rv_cfg["Md5"] = "\n".join(lines)
                        cfg["ResVersionConfig"] = rv_cfg
                        modified = True
                        break

        if modified:
            with open(file_path, "w") as f:
                json.dump(cfg, f, indent=2)


def get_json_files(targets):
    files = []
    for t in targets:
        if os.path.isfile(t):
            if t.endswith(".json"):
                files.append(t)
        elif os.path.isdir(t):
            for root, _, fns in os.walk(t):
                for fn in sorted(fns):
                    if fn.endswith(".json"):
                        files.append(os.path.join(root, fn))
    return files


def parse_label(path):
    parts = path.replace("\\", "/").split("/")
    area = plat = None
    for i, p in enumerate(parts):
        if p in ("CNREL", "OSREL"):
            area = p
            if i + 1 < len(parts):
                plat = parts[i + 1]
            break
    version = os.path.splitext(os.path.basename(path))[0]
    return area, plat, version


def main():
    print("=" * 60)
    print("热更配置校验工具 (高并发 + 自动修复)")
    script_dir = os.path.dirname(os.path.abspath(__file__))
    print(f"配置目录: {script_dir}")
    print(f"保存目录: {DATA_ROOT}")
    print(f"并发数:   {MAX_WORKERS}")
    print("=" * 60)

    targets = sys.argv[1:] if len(sys.argv) > 1 else [script_dir]
    json_files = get_json_files(targets)
    if not json_files:
        print("未找到配置文件")
        return

    all_tasks = []
    all_labels = set()
    for path in json_files:
        with open(path) as f:
            cfg = json.load(f)
        area, plat, version = parse_label(path)
        label = f"{area}/{plat}/{version}"
        all_labels.add(label)
        tasks = collect_tasks(cfg, area, plat, version, path)
        all_tasks.extend(tasks)

    total_files = len(all_tasks)
    total_versions = len(all_labels)
    print(f"配置数: {total_versions}  文件数: {total_files}")
    print()

    fail_entries = {}
    fixes = []
    done = 0

    with ThreadPoolExecutor(max_workers=MAX_WORKERS) as pool:
        fut_map = {pool.submit(process_one, t): t for t in all_tasks}
        for fut in as_completed(fut_map):
            done += 1
            label, err, fix = fut.result()
            if err and not err.startswith("patch_node_versions"):
                fail_entries.setdefault(label, set()).add(err)
            if fix:
                fixes.append(fix)
            pct = done * 100 // total_files
            sys.stdout.write(f"\r  下载中: {done}/{total_files} ({pct}%)                      ")
            sys.stdout.flush()

    print()
    print()

    if fixes:
        print("  ── 修复配置中 ...")
        apply_fixes(fixes)
        print(f"  已修复 {len(fixes)} 处不匹配")



    ok_set = set()
    for label in all_labels:
        if label not in fail_entries:
            ok_set.add(label)

    print()
    print("=" * 60)
    print(f"总计: {len(all_labels)}  通过: {len(ok_set)}  失败: {len(fail_entries)}")

    if fail_entries:
        print()
        print("以下版本存在异常:")
        for label in sorted(fail_entries):
            print(f"  ✗ {label}")
            for err in sorted(fail_entries[label]):
                print(f"      - {err}")
    else:
        if all_labels:
            print("所有版本均正常 ✓")
    print("=" * 60)


if __name__ == "__main__":
    main()
