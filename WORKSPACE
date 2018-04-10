GITHUB_REPOS = {
    "io_bazel_rules_go": ("znly", "rules_go", "908fd9c70af2c78d70891263bb7d7b2651647b35"),
    "bazel_gazelle": ("bazelbuild", "bazel-gazelle", "db967cc738fb9cc1f081461b531c525dea57b2a0"),
}

[
    http_archive(
        name = name,
        urls = ["https://codeload.github.com/%s/%s/tar.gz/%s" % (username, project, commit)],
        strip_prefix = "%s-%s" % (project, commit),
        type = "tar.gz",
    )
    for name, (username, project, commit) in GITHUB_REPOS.items()
]

load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains(go_version = "1.10")
