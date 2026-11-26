from union import workflow, task


@task
def say_hi(i: str):
    print(f"Hii {i}")
