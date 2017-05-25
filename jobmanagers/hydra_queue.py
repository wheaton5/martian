#!/usr/bin/env python
#
# Copyright (c) 2017 10X Genomics, Inc. All rights reserved.
#

"""Queries the hydra spool about a list of jobs and parses the output,
returning on stdout the subset of the jobs given on standard in which are
queued, claimed, or running."""


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

    def query_backing(self):
        """Removes jobs which are unknown to hydra from the set."""
        jobs = os.listdir(os.path.join(self._hydra_root, '.job_backing'))
        self.intersection_update(jobs)

    def query_success(self):
        """Removes jobs which have finished successfully."""
        jobs = os.listdir(os.path.join(self._hydra_root, 'archive', 'succeeded'))
        self.difference_update(jobs)

    def query_fail(self):
        """Removes jobs which have failed."""
        jobs = os.listdir(os.path.join(self._hydra_root, 'archive', 'failed'))
        self.difference_update(jobs)


def query_hydra(ids):
    """Returns the subset of ids which are pending or running."""
    if not ids:
        return []
    remaining = _JobidSet(ids)
    # The most likely cases of jobs which are not running where the runtime
    # thinks they are is jobs which have failed.  Remove those first and see
    # whether any are left.
    remaining.query_fail()
    if not remaining:
        return []
    # Jobs which have not been cleaned out the succeeded or failed archive by
    # the janitor will be found in the backing store.  If they aren't there,
    # they're certainly not queued or running.
    remaining.query_backing()
    if not remaining:
        return []
    remaining.query_success()
    return remaining


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
