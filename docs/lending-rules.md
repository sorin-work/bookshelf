# How lending works

Everything here is decided by the lending rules on the **Settings** page.
The numbers in brackets are the defaults for a new library.

All dates are calendar days in the library's time zone (`LIBRARY_TIMEZONE`).

## Checking out

A member can borrow when:

- their account is open and their library card hasn't expired,
- they have fewer items out than the limit (5),
- they owe less than the borrowing block (10.00) in unpaid fines.

The copy has to be on the shelf, or on the hold shelf for that member.

The due date is the loan period (21 days) after today. If the library is
closed that day (Sundays, or one of the closure dates such as a bank
holiday) the due date moves to the next day it is open.

## Renewing

A renewal sets the due date to a full loan period from today, worked out
the same way as at checkout, but never earlier than the current due date.
Members renew from their account page (one at a time or all at once);
librarians from the member page or by scanning the copy at the desk.

A loan can't be renewed when:

- it is already overdue (bring it to the desk instead),
- it has been renewed the maximum number of times (2),
- someone is waiting in the hold queue for the title,
- the member's library card has expired.

Holds that already have a copy waiting on the hold shelf don't block
renewals: that member has their copy.

## Holds

Members can place a hold only when every copy of a title is out. Lost and
withdrawn copies don't count, and a title with no copies can't be held.
Each member can have a limited number of open holds (5); setting the limit
to 0 turns holds off.

The queue is first come, first served. When a copy is checked in and
someone is waiting, the desk is told to put it on the hold shelf for the
first member in the queue. It waits for them for the pickup period (7
days), ending on a day the library is open. Uncollected holds expire once
an hour; the copy then goes to the next member in the queue, or back on
the shelf if nobody is waiting.

Checking out any copy of a title closes the borrower's own hold on it.
Librarians can also place a hold for a member from the title page, by
card number.

The title page gives a rough idea of the wait: each copy serves one member
per loan period, so with two copies the third person in the queue can
expect to wait about two loan periods. Returns are often earlier and
renewals make it later, so it is only a guide.

If the last copy of a title is withdrawn while members are still waiting,
their holds are cancelled because they can never be filled.

## Fines

A late return costs the daily fine (0.25) for every day after the due
date, up to the cap per item (5.00). With grace days set, items returned
within that many days late cost nothing; after that every late day counts.

Fines are added when the item comes back. A book reported lost ends the
loan without a late fine; the librarian can add a replacement charge
instead. The overdue list shows what an
item would cost if it came back today. Librarians can also add charges by
hand (for a damaged or lost book), take payments and waive fines on the
member page.
