# sportys-test-scraper

# What is This?
Automatically generate new practice tests from your previously missed questions on your past Sporty's practice tests (in PDF format). 
See statistics like how long you took on each test vs. the allowed time limit on the real test, and how many questions you got wrong per test.

This tool is mainly for people who understand how to write software. I'm not going to release "easier" versions for people who don't know how to do things like set environment variables, inspect requests, etc.

# Usage
1. Download the latest release from the releases page.
2. 
2. Set two environment variables:
    - JWT_TOKEN (Open developer tools in Chrome, find this after the word "Bearer")
    - COURSE_TYPE (Possible values: PRIVATE or INSTRUMENT)
2. Run ./sportys_test_scraper