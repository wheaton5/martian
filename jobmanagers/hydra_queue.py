#!/usr/bin/env python
#
# Copyright (c) 2017 10X Genomics, Inc. All rights reserved.
#

"""Queries the hydra spool about a list of jobs and parses the output,
returning on stdout the subset of the jobs given on standard in which are in
the queued state."""


import errno
import os
import os.path
import sys


def get_ids():
    """Returns he set of jobids to query from standard input."""
    ids = []
    for jobid in sys.stdin.readlines():
        ids.append(jobid.strip())
    return ids


class _JobidSet(set):
    """A set of job ids that knows how to query the hydra spool to cull out ids
    which are not pending."""

    def __init__(self, ids, root=os.getenv('HYDRA_ROOT')):
        self._hydra_root = root
        super(_JobidSet, self).__init__(ids)

    def query_pending(self):
        """Removes pending items from the set."""
        pending = os.listdir(os.path.join(self._hydra_root, 'pending'))
        for item in pending:
            try:
                self.remove(item)
            except KeyError:
                pass

    def query_claimed(self):
        """Removes items which are claimed by hosts."""
        for host in os.listdir(os.path.join(self._hydra_root, 'hosts')):
            try:
                claimed = os.listdir(os.path.join(host, 'claimed'))
            except OSError as err:
                if err.errno != errno.ENOENT:
                    raise
                else:
                    continue
            for item in claimed:
                try:
                    self.remove(item)
                except KeyError:
                    pass
            if len(self) == 0:
                return

    def query_archive(self):
        """Removes items which are claimed by archived hosts."""
        for host in os.listdir(os.path.join(self._hydra_root,
                                            'archive', 'hosts')):
            try:
                claimed = os.listdir(os.path.join(host, 'claimed'))
            except OSError as err:
                if err.errno != errno.ENOENT:
                    raise
                else:
                    continue
            for item in claimed:
                try:
                    self.remove(item)
                except KeyError:
                    pass
            if len(self) == 0:
                return


def find_not_pending(ids):
    """Returns the subset of ids which are not still pending."""
    if not ids:
        return []
    remaining = _JobidSet(ids)
    remaining.query_pending()
    if not remaining:
        return []
    remaining.query_claimed()
    if not remaining:
        return []
    remaining.query_archive()
    if not remaining:
        return []
    return remaining


def query_hydra(ids):
    """Returns the subset of ids which are still pending."""
    id_set = set(ids)
    not_pending = find_not_pending(id_set)
    return id_set - not_pending


def main():
    """Reads a set of ids from standard input, queries qstat, and outputs the
    jobids to standard output for jobs which are in the pending state."""
    hydra_root = os.getenv('HYDRA_ROOT')
    if not hydra_root:
        raise Exception('Hydra root not set.')
    for jobid in query_hydra(get_ids()):
        sys.stdout.write('%s\n' % jobid)
    return 0

if __name__ == '__main__':
    sys.exit(main())
