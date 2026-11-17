const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '../../BeamNG-Mod/ui/ui-vue/mods/dualsensehaptics');
const read = name => fs.readFileSync(path.join(root, name), 'utf8');

test('BeamNG-style tabs expose selectable accessible panels and keyboard behavior', () => {
  const vue = read('DualSenseHapticsSettings.vue');
  assert.match(vue, /role="tablist"/);
  for (const tab of ['haptics', 'inputs', 'speaker']) {
    assert.match(vue, new RegExp(`id="dph-tab-${tab}"`));
    assert.match(vue, new RegExp(`id="dph-panel-${tab}"`));
  }
  assert.match(vue, /function onTabKeydown\(event\)/);
  assert.match(vue, /:aria-selected=/);
  assert.match(vue, /:aria-expanded="advancedSurfacesOpen"/);
});

test('disabled parent controls never commit changes and range input remains bounded', () => {
  const slider = read('SettingSlider.vue');
  const range = read('SettingRangeSlider.vue');
  assert.match(slider, /:disabled="disabled"/);
  assert.match(slider, /!props\.disabled && Number\.isFinite/);
  assert.match(range, /if \(!props\.disabled\) emit/);
  assert.doesNotMatch(range, /Math\.min\(90/);
  assert.doesNotMatch(range, /:disabled="disabled"\s+:disabled="disabled"/);
  assert.match(read('DualSenseHapticsSettings.vue'), /:disabled="!values\.lighting\.enabled"/);
});

test('section headings, feature labels and sliders keep distinct type and spacing scales', () => {
  const main = read('DualSenseHapticsSettings.vue');
  const slider = read('SettingSlider.vue');
  const range = read('SettingRangeSlider.vue');
  assert.match(main, /\.group h3\s*\{[^}]*font-size:\s*1\.13rem/s);
  assert.match(main, /\.feature-title\s*\{[^}]*font-size:\s*1\.01rem/s);
  assert.match(slider, /\.setting-title\s*\{[^}]*font-size:\s*0\.91rem/s);
  assert.match(slider, /\.setting-slider\.compact\s*\{[^}]*margin:\s*1\.06rem/s);
  assert.match(range, /\.setting-range\s*\{[^}]*margin:\s*1\.16rem/s);
  assert.match(main, /\.dph-settings \.embedded-panel \.group h3\s*\{[^}]*font-size:\s*1\.13rem/s);
});

test('slider labels have consistent breathing room after preceding settings', () => {
  const slider = read('SettingSlider.vue');
  const range = read('SettingRangeSlider.vue');
  assert.match(slider, /\.setting-slider\s*\{[^}]*margin:\s*1\.18rem\s+0\s+0\.64rem/s);
  assert.match(slider, /\.setting-slider\.compact\s*\{[^}]*margin:\s*1\.06rem\s+0\s+0\.54rem/s);
  assert.match(range, /\.setting-range\s*\{[^}]*margin:\s*1\.16rem\s+0\s+0\.6rem/s);
  assert.match(range, /\.setting-range\.compact\s*\{[^}]*margin-top:\s*1\.06rem/s);
});
