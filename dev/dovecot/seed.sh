#!/bin/sh
set -eu
umask 077

home=/srv/vmail/dev@example.test
maildir="$home/Maildir"
mkdir -p "$maildir/cur" "$maildir/new" "$maildir/tmp"
# The official rootless Dovecot image uses UID and GID 1000 for vmail.
chown 1000:1000 /srv/vmail "$home" "$maildir" "$maildir/cur" "$maildir/new" "$maildir/tmp"
chmod 0700 /srv/vmail "$home" "$maildir" "$maildir/cur" "$maildir/new" "$maildir/tmp"

for fixture in /fixtures/*.eml; do
    [ -f "$fixture" ] || continue
    filename=${fixture##*/}
    filename=${filename%.eml}
    # Dovecot can move messages to cur and append size and flag suffixes.
    for file in "$maildir"/new/"$filename" "$maildir"/new/"$filename,"* "$maildir"/new/"$filename:"* \
        "$maildir"/cur/"$filename" "$maildir"/cur/"$filename,"* "$maildir"/cur/"$filename:"*; do
        [ ! -e "$file" ] || continue 2
    done

    temporary=$(mktemp "$maildir/tmp/fixture.XXXXXX")
    cp "$fixture" "$temporary"
    chown 1000:1000 "$temporary"
    mv "$temporary" "$maildir/new/$filename"
done
