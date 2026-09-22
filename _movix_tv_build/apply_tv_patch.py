#!/usr/bin/env python3
from pathlib import Path
import shutil
import sys

BUNDLE = Path(__file__).resolve().parent
REPO = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else Path.cwd().resolve()
APP = REPO / "app"

if not (APP / "package.json").exists():
    raise SystemExit("Repo Movix introuvable: lance ce script depuis la racine de MovixOpenSource ou passe le chemin du repo en argument.")


def replace_once(path: Path, old: str, new: str) -> None:
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"Anchor inattendu dans {path}: attendu 1 occurrence, trouvé {count}")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")
    print(f"patched {path.relative_to(REPO)}")


def copy_overlay(rel: str) -> None:
    src = BUNDLE / "overlay" / rel
    dst = REPO / rel
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, dst)
    print(f"added   {rel}")

copy_overlay("app/src/tv/tvWebView.ts")
copy_overlay("app/android/app/src/main/res/drawable-nodpi/tv_banner.png")

p = APP / "src/components/WebViewBrowser.tsx"
replace_once(p,"import { CONFIG } from '../config';\n","import { CONFIG } from '../config';\nimport {\n  TV_NAVIGATION_SCRIPT,\n  TV_OPEN_SETTINGS_MESSAGE,\n  TV_USER_AGENT,\n  isTvRuntimeMessage,\n} from '../tv/tvWebView';\n")
replace_once(p,"  onPictureInPictureModeChange?: (active: boolean) => void;\n}","  onPictureInPictureModeChange?: (active: boolean) => void;\n  onOpenTvSettings?: () => void;\n}")
replace_once(p,"  ({ url, onNavigationStateChange, onError, onPictureInPictureModeChange }, ref) => {","  ({ url, onNavigationStateChange, onError, onPictureInPictureModeChange, onOpenTvSettings }, ref) => {")
replace_once(p,"    const onMessage = useCallback((event: WebViewMessageEvent) => {\n      const isTopFrame = typeof event.nativeEvent.isTopFrame === 'boolean'","    const onMessage = useCallback((event: WebViewMessageEvent) => {\n      const rawMessage = event.nativeEvent.data;\n      if (Platform.isTV && isTvRuntimeMessage(rawMessage, TV_OPEN_SETTINGS_MESSAGE)) {\n        onOpenTvSettings?.();\n        return;\n      }\n\n      const isTopFrame = typeof event.nativeEvent.isTopFrame === 'boolean'")
replace_once(p,"      handleBridgeMessage(event.nativeEvent.data, webViewRef, {","      handleBridgeMessage(rawMessage, webViewRef, {")
replace_once(p,"    }, [url]);\n\n    // `window.open`","    }, [onOpenTvSettings, url]);\n\n    // `window.open`")
replace_once(p,"    const injectedJS = useMemo(\n      () => injectedJavaScriptFor(journalConsole),\n      [journalConsole],\n    );","    const injectedJS = useMemo(() => {\n      const base = injectedJavaScriptFor(journalConsole);\n      return Platform.isTV ? `${base}\\n${TV_NAVIGATION_SCRIPT}` : base;\n    }, [journalConsole]);")
replace_once(p,"    const userAgent = Platform.OS === 'ios' ? undefined : CONFIG.USER_AGENT;","    const userAgent = Platform.OS === 'ios'\n      ? undefined\n      : Platform.isTV\n        ? TV_USER_AGENT\n        : CONFIG.USER_AGENT;")
replace_once(p,"        contentMode=\"mobile\"","        contentMode={Platform.isTV ? 'desktop' : 'mobile'}\n        textZoom={100}")

p = APP / "src/screens/BrowserScreen.tsx"
replace_once(p,"  const insets = useSafeAreaInsets();\n  const webViewRef","  const insets = useSafeAreaInsets();\n  const isTV = Platform.isTV;\n  const webViewRef")
replace_once(p,"  const toolbarHidden = !uiPrefs.showUrlBar && !uiPrefs.showNavBar;","  const toolbarHidden = isTV || (!uiPrefs.showUrlBar && !uiPrefs.showNavBar);")
replace_once(p,"          onPictureInPictureModeChange={onPictureInPictureModeChange}\n        />","          onPictureInPictureModeChange={onPictureInPictureModeChange}\n          onOpenTvSettings={() => setSettingsVisible(true)}\n        />")
replace_once(p,"        animationType=\"slide\"","        animationType={isTV ? 'fade' : 'slide'}")
replace_once(p,"      {!isPictureInPictureActive && navBarHidden && (\n        <MiniPill","      {!isTV && !isPictureInPictureActive && navBarHidden && (\n        <MiniPill")

p = APP / "src/services/addressResolver.ts"
replace_once(p,"import { UPDATE_CHECK, FALLBACK_CONFIG } from '../config';\n","import AsyncStorage from '@react-native-async-storage/async-storage';\nimport { UPDATE_CHECK, FALLBACK_CONFIG } from '../config';\n")
replace_once(p,"type RawMirror = { url?: unknown };","const ADDRESS_CACHE_KEY = 'movix:address:last-known-good:v1';\n\ntype RawMirror = { url?: unknown };")
cache_helpers = r'''

type CachedAddress = {
  savedAt: number;
  config: AddressConfig;
};

function isHttpsUrl(value: string): boolean {
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'https:' && Boolean(parsed.hostname);
  } catch {
    return false;
  }
}

function validateCachedConfig(value: unknown): AddressConfig | null {
  if (!value || typeof value !== 'object') return null;
  const candidate = value as Partial<AddressConfig>;
  if (!isString(candidate.primaryUrl) || !isHttpsUrl(candidate.primaryUrl)) return null;
  if (!Array.isArray(candidate.mirrors)) return null;
  if (!isString(candidate.githubUrl) || !isHttpsUrl(candidate.githubUrl)) return null;
  if (!isString(candidate.telegramUrl) || !isHttpsUrl(candidate.telegramUrl)) return null;
  const mirrors = candidate.mirrors.filter(
    (url): url is string => isString(url) && isHttpsUrl(url) && url !== candidate.primaryUrl,
  );
  return {
    primaryUrl: candidate.primaryUrl,
    mirrors,
    githubUrl: candidate.githubUrl,
    telegramUrl: candidate.telegramUrl,
  };
}

async function readCachedAddress(): Promise<AddressConfig | null> {
  try {
    const raw = await AsyncStorage.getItem(ADDRESS_CACHE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<CachedAddress>;
    return validateCachedConfig(parsed.config);
  } catch {
    return null;
  }
}

async function writeCachedAddress(config: AddressConfig): Promise<void> {
  try {
    const payload: CachedAddress = { savedAt: Date.now(), config };
    await AsyncStorage.setItem(ADDRESS_CACHE_KEY, JSON.stringify(payload));
  } catch {
  }
}
'''
replace_once(p,"const HARDCODED_FALLBACK: AddressConfig = {\n  primaryUrl: FALLBACK_CONFIG.PRIMARY_URL,\n  mirrors: [],\n  githubUrl: FALLBACK_CONFIG.GITHUB_URL,\n  telegramUrl: FALLBACK_CONFIG.TELEGRAM_URL,\n};\n","const HARDCODED_FALLBACK: AddressConfig = {\n  primaryUrl: FALLBACK_CONFIG.PRIMARY_URL,\n  mirrors: [],\n  githubUrl: FALLBACK_CONFIG.GITHUB_URL,\n  telegramUrl: FALLBACK_CONFIG.TELEGRAM_URL,\n};\n" + cache_helpers)
replace_once(p,"    console.warn('[addressResolver] rentry fetch failed', err);\n    return HARDCODED_FALLBACK;","    console.warn('[addressResolver] rentry fetch failed', err);\n    return (await readCachedAddress()) ?? HARDCODED_FALLBACK;")
replace_once(p,"    if (!normalized) throw new Error('address.json: invalid shape');\n    return normalized;","    if (!normalized) throw new Error('address.json: invalid shape');\n    await writeCachedAddress(normalized);\n    return normalized;")
replace_once(p,"    console.warn('[addressResolver] address.json fetch failed', err);\n    return HARDCODED_FALLBACK;","    console.warn('[addressResolver] address.json fetch failed', err);\n    return (await readCachedAddress()) ?? HARDCODED_FALLBACK;")

p = APP / "src/App.tsx"
replace_once(p,"  const { state, accept, dismiss, cancel, openSettings, retry } = useAppUpdate(\n    config?.githubUrl ?? null,\n  );","  const { state, accept, dismiss, cancel, openSettings, retry } = useAppUpdate(\n    Platform.isTV ? null : (config?.githubUrl ?? null),\n  );")
replace_once(p,"      <StatusBar barStyle=\"light-content\" backgroundColor=\"#0a0a0a\" />","      <StatusBar\n        hidden={Platform.isTV}\n        barStyle=\"light-content\"\n        backgroundColor=\"#0a0a0a\"\n      />")

p = APP / "android/app/src/main/AndroidManifest.xml"
replace_once(p,"<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\">\n","<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\">\n\n    <uses-feature android:name=\"android.software.leanback\" android:required=\"false\" />\n    <uses-feature android:name=\"android.hardware.touchscreen\" android:required=\"false\" />\n")
replace_once(p,"        android:roundIcon=\"@mipmap/ic_launcher_round\"\n        android:allowBackup","        android:roundIcon=\"@mipmap/ic_launcher_round\"\n        android:banner=\"@drawable/tv_banner\"\n        android:allowBackup")
replace_once(p,"                <category android:name=\"android.intent.category.LAUNCHER\" />","                <category android:name=\"android.intent.category.LAUNCHER\" />\n                <category android:name=\"android.intent.category.LEANBACK_LAUNCHER\" />")
for short_name, full_name in [
    ('android:name=\".MainApplication\"','android:name=\"com.movix.app.MainApplication\"'),
    ('android:name=\".MainActivity\"','android:name=\"com.movix.app.MainActivity\"'),
    ('android:name=\".dns.DnsVpnService\"','android:name=\"com.movix.app.dns.DnsVpnService\"'),
    ('android:name=\".cast.CastProxyForegroundService\"','android:name=\"com.movix.app.cast.CastProxyForegroundService\"'),
    ('android:name=\".pip.PictureInPictureActionReceiver\"','android:name=\"com.movix.app.pip.PictureInPictureActionReceiver\"'),
]:
    replace_once(p, short_name, full_name)

p = APP / "android/app/build.gradle"
replace_once(p,'        applicationId "com.movix.app"','        applicationId "com.movix.app.tv"')
replace_once(p,'        versionName "2.6.1"','        versionName "2.6.1-tv.1"')

p = APP / "android/app/src/main/res/values/strings.xml"
replace_once(p,'<string name="app_name">Movix</string>','<string name="app_name">Movix TV</string>')

print("Movix TV patch applied successfully.")
