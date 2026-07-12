from corthena.compatibility.storage import run_storage_probe


def test_storage_and_published_matrix() -> None:
    evidence = run_storage_probe()
    assert evidence.rows == 1
    assert evidence.arrow_bytes > 0
    assert evidence.matrix_sum == 10.0
