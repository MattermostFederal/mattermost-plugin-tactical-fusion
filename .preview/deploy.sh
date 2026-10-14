TF_REPO=MattermostFederal/mattermost-plugin-tactical-fusion
TF_DATA_HOST=docker/mattermost/data
TF_DATA_PATH=/mattermost/data
MATTERMOST_CONTAINER_UID=2000

tf_release_base() {
  local tag
  tag="$(curl -fsSL --retry 3 "https://api.github.com/repos/${TF_REPO}/releases/latest" | jq -r '.tag_name // empty')" || return 1
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
  printf 'https://github.com/%s/releases/download/%s' "$TF_REPO" "$tag"
}

tf_fetch_release_set() {
  local base="$1" sums="$2" dir="$TF_DATA_HOST/$3" list sha name have=0
  list="$(curl -fsSL --retry 3 "$base/$sums")" || return 1
  install -d -m 0755 "$dir"
  touch "$dir/.verified"
  while read -r sha name; do
    [[ "$sha" =~ ^[0-9a-f]{64}$ && "$name" =~ ^[A-Za-z0-9._-]+\.(pmtiles|tsv\.gz|mmdb)$ ]] || continue
    if [ -f "$dir/$name" ] && grep -qxF "$sha  $name" "$dir/.verified"; then
      have=$((have + 1))
      continue
    fi
    if curl -fsSL --retry 3 -o "$dir/$name.part" "$base/$name" &&
      echo "$sha  $dir/$name.part" | sha256sum -c --quiet -; then
      mv -f "$dir/$name.part" "$dir/$name"
      awk -v n="$name" '$2 != n' "$dir/.verified" > "$dir/.verified.next"
      echo "$sha  $name" >> "$dir/.verified.next"
      mv -f "$dir/.verified.next" "$dir/.verified"
      echo "fetched $name"
      have=$((have + 1))
    else
      rm -f "$dir/$name.part"
      echo "warning: could not fetch $name"
    fi
  done <<<"$list"
  chown -R "$MATTERMOST_CONTAINER_UID:$MATTERMOST_CONTAINER_UID" "$dir"
  [ "$have" -gt 0 ]
}

tf_settings='{}'
if tf_base="$(tf_release_base)"; then
  if tf_fetch_release_set "$tf_base" PACKAGES.sha256 map-packages; then
    tf_settings="$(jq -c --arg p "$TF_DATA_PATH/map-packages" '. + {locationmappackagesdir: $p}' <<<"$tf_settings")"
  else
    echo "warning: no map packages from ${tf_base##*/}"
  fi
  if tf_fetch_release_set "$tf_base" DATASETS.sha256 cyber-datasets; then
    tf_settings="$(jq -c --arg p "$TF_DATA_PATH/cyber-datasets" '. + {cyberdatasetsdir: $p}' <<<"$tf_settings")"
  else
    echo "warning: no cyber datasets from ${tf_base##*/}"
  fi
else
  echo "warning: could not find the latest Tactical Fusion release"
fi

if [ "$tf_settings" != '{}' ]; then
  jq -nc --arg id "$id" --argjson s "$tf_settings" '{PluginSettings: {Plugins: {($id): $s}}}' \
    > "$TF_DATA_HOST/.preview-patch.json"
  if mmctl config patch "$TF_DATA_PATH/.preview-patch.json" >/dev/null; then
    echo "configured ${id}: $(jq -r 'to_entries | map("\(.key)=\(.value)") | join(", ")' <<<"$tf_settings")"
  else
    echo "warning: could not configure ${id}"
  fi
  rm -f "$TF_DATA_HOST/.preview-patch.json"
fi
