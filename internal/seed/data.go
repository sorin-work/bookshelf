package seed

type book struct {
	isbn      string
	title     string
	subtitle  string
	authors   []string
	subjects  []string
	publisher string
	year      int
	copies    int
	shelf     string
	about     string
}

var books = []book{
	{"9780141439518", "Pride and Prejudice", "", []string{"Jane Austen"}, []string{"Classics", "Romance"}, "Penguin Classics", 2002, 2, "Classics A",
		"Elizabeth Bennet and Mr Darcy misjudge each other across a string of balls, visits and proposals."},
	{"9780451524935", "1984", "", []string{"George Orwell"}, []string{"Classics", "Dystopia"}, "Signet Classics", 1961, 3, "Classics O",
		"Winston Smith works at the Ministry of Truth and starts, quietly, to doubt."},
	{"9780061120084", "To Kill a Mockingbird", "", []string{"Harper Lee"}, []string{"Classics"}, "Harper Perennial", 2006, 2, "Classics L", ""},
	{"9780743273565", "The Great Gatsby", "", []string{"F. Scott Fitzgerald"}, []string{"Classics"}, "Scribner", 2004, 2, "Classics F", ""},
	{"9780547928227", "The Hobbit", "or There and Back Again", []string{"J. R. R. Tolkien"}, []string{"Fantasy", "Classics"}, "Houghton Mifflin Harcourt", 2012, 3, "Fantasy T",
		"Bilbo Baggins is swept off on an adventure with a wizard and thirteen dwarves."},
	{"9780618640157", "The Lord of the Rings", "", []string{"J. R. R. Tolkien"}, []string{"Fantasy"}, "Houghton Mifflin", 2005, 1, "Fantasy T", ""},
	{"9780439708180", "Harry Potter and the Sorcerer's Stone", "", []string{"J. K. Rowling"}, []string{"Fantasy", "Children"}, "Scholastic", 1998, 3, "Children R", ""},
	{"9780142437247", "Moby-Dick", "or, The Whale", []string{"Herman Melville"}, []string{"Classics", "Adventure"}, "Penguin Classics", 2003, 1, "Classics M", ""},
	{"9780140449136", "Crime and Punishment", "", []string{"Fyodor Dostoevsky"}, []string{"Classics"}, "Penguin Classics", 2003, 1, "Classics D", ""},
	{"9780441172719", "Dune", "", []string{"Frank Herbert"}, []string{"Science fiction"}, "Ace", 1990, 2, "SF H",
		"On the desert planet Arrakis, Paul Atreides is caught up in a war over the most valuable substance in the universe."},
	{"9780316769488", "The Catcher in the Rye", "", []string{"J. D. Salinger"}, []string{"Classics"}, "Little, Brown", 1991, 1, "Classics S", ""},
	{"9780060850524", "Brave New World", "", []string{"Aldous Huxley"}, []string{"Classics", "Dystopia"}, "Harper Perennial", 2006, 1, "Classics H", ""},
	{"9780142410318", "Charlie and the Chocolate Factory", "", []string{"Roald Dahl"}, []string{"Children"}, "Puffin", 2007, 2, "Children D", ""},
	{"9780142410370", "Matilda", "", []string{"Roald Dahl"}, []string{"Children"}, "Puffin", 2007, 2, "Children D", ""},
	{"9780439023528", "The Hunger Games", "", []string{"Suzanne Collins"}, []string{"Young adult", "Dystopia"}, "Scholastic", 2010, 2, "YA C", ""},
	{"9780375842207", "The Book Thief", "", []string{"Markus Zusak"}, []string{"Young adult", "Historical fiction"}, "Knopf", 2007, 1, "YA Z", ""},
	{"9780062315007", "The Alchemist", "", []string{"Paulo Coelho"}, []string{"Fiction"}, "HarperOne", 2014, 2, "Fiction C", ""},
	{"9780385490818", "The Handmaid's Tale", "", []string{"Margaret Atwood"}, []string{"Fiction", "Dystopia"}, "Anchor", 1998, 1, "Fiction A", ""},
	{"9780307277671", "The Road", "", []string{"Cormac McCarthy"}, []string{"Fiction"}, "Vintage", 2007, 1, "Fiction M", ""},
	{"9781400033416", "Beloved", "", []string{"Toni Morrison"}, []string{"Fiction", "Historical fiction"}, "Vintage", 2004, 1, "Fiction M", ""},
	{"9780735219090", "Where the Crawdads Sing", "", []string{"Delia Owens"}, []string{"Fiction", "Mystery"}, "G. P. Putnam's Sons", 2018, 2, "Fiction O", ""},
	{"9780525559474", "The Midnight Library", "", []string{"Matt Haig"}, []string{"Fiction"}, "Viking", 2020, 2, "Fiction H",
		"Between life and death there is a library, and every book in it is a life Nora could have lived."},
	{"9780593135204", "Project Hail Mary", "", []string{"Andy Weir"}, []string{"Science fiction"}, "Ballantine Books", 2021, 2, "SF W", ""},
	{"9780553418026", "The Martian", "", []string{"Andy Weir"}, []string{"Science fiction"}, "Broadway Books", 2014, 1, "SF W", ""},
	{"9780062316097", "Sapiens", "A Brief History of Humankind", []string{"Yuval Noah Harari"}, []string{"History", "Non-fiction"}, "Harper", 2015, 2, "900 HAR", ""},
	{"9780374533557", "Thinking, Fast and Slow", "", []string{"Daniel Kahneman"}, []string{"Psychology", "Non-fiction"}, "Farrar, Straus and Giroux", 2013, 1, "150 KAH", ""},
	{"9780399590504", "Educated", "A Memoir", []string{"Tara Westover"}, []string{"Memoir", "Non-fiction"}, "Random House", 2018, 2, "920 WES", ""},
	{"9780064400558", "Charlotte's Web", "", []string{"E. B. White"}, []string{"Children", "Classics"}, "HarperTrophy", 2001, 2, "Children W", ""},
	{"9780394800011", "The Cat in the Hat", "", []string{"Dr. Seuss"}, []string{"Children", "Picture books"}, "Random House", 1957, 2, "Picture S", ""},
	{"9780399226908", "The Very Hungry Caterpillar", "", []string{"Eric Carle"}, []string{"Children", "Picture books"}, "Philomel", 1994, 2, "Picture C", ""},
	{"9780060254926", "Where the Wild Things Are", "", []string{"Maurice Sendak"}, []string{"Children", "Picture books"}, "HarperCollins", 1988, 1, "Picture S", ""},
	{"9780316015844", "Twilight", "", []string{"Stephenie Meyer"}, []string{"Young adult", "Romance"}, "Little, Brown", 2006, 1, "YA M", ""},
	{"9781250301697", "The Silent Patient", "", []string{"Alex Michaelides"}, []string{"Mystery", "Thriller"}, "Celadon Books", 2019, 2, "Mystery M", ""},
	{"9780525536291", "The Vanishing Half", "", []string{"Brit Bennett"}, []string{"Fiction"}, "Riverhead Books", 2020, 1, "Fiction B", ""},
	{"9780140177398", "Of Mice and Men", "", []string{"John Steinbeck"}, []string{"Classics"}, "Penguin", 1993, 2, "Classics S", ""},
	{"9780765326355", "The Way of Kings", "", []string{"Brandon Sanderson"}, []string{"Fantasy"}, "Tor", 2010, 1, "Fantasy S", ""},
	{"9780756404741", "The Name of the Wind", "", []string{"Patrick Rothfuss"}, []string{"Fantasy"}, "DAW", 2008, 1, "Fantasy R", ""},
	{"9780345391803", "The Hitchhiker's Guide to the Galaxy", "", []string{"Douglas Adams"}, []string{"Science fiction", "Humour"}, "Del Rey", 1995, 2, "SF A", ""},
	{"9780553293357", "Foundation", "", []string{"Isaac Asimov"}, []string{"Science fiction", "Classics"}, "Bantam Spectra", 1991, 1, "SF A", ""},
	{"9780441569595", "Neuromancer", "", []string{"William Gibson"}, []string{"Science fiction"}, "Ace", 1984, 1, "SF G", ""},
	{"9780307743657", "The Shining", "", []string{"Stephen King"}, []string{"Horror"}, "Anchor", 2012, 1, "Fiction K", ""},
	{"9780062073488", "And Then There Were None", "", []string{"Agatha Christie"}, []string{"Mystery", "Classics"}, "William Morrow", 2011, 2, "Mystery C", ""},
	{"9780486282114", "Frankenstein", "", []string{"Mary Shelley"}, []string{"Classics", "Horror"}, "Dover", 1994, 1, "Classics S", ""},
	{"9780141439600", "A Tale of Two Cities", "", []string{"Charles Dickens"}, []string{"Classics", "Historical fiction"}, "Penguin Classics", 2003, 1, "Classics D", ""},
	{"9780141441146", "Jane Eyre", "", []string{"Charlotte Brontë"}, []string{"Classics", "Romance"}, "Penguin Classics", 2006, 1, "Classics B", ""},
	{"9780141439556", "Wuthering Heights", "", []string{"Emily Brontë"}, []string{"Classics"}, "Penguin Classics", 2003, 1, "Classics B", ""},
	{"9780399501487", "Lord of the Flies", "", []string{"William Golding"}, []string{"Classics"}, "Perigee", 1959, 2, "Classics G", ""},
	{"", "A Hundred Years on Mill Street", "Stories from the parish", []string{"Ileana Popescu", "Tom Hardwick"}, []string{"Local history"}, "Mill Street Press", 2019, 1, "Local", "Collected memories of the families who lived on Mill Street, donated by the authors."},
}

type person struct {
	name  string
	email string
	phone string
}

var people = []person{
	{"Ana Ionescu", "ana.ionescu@example.com", "0722 410 118"},
	{"Mihai Popa", "mihai.popa@example.com", ""},
	{"Sarah Whitfield", "sarah.w@example.com", "07700 900 341"},
	{"Andrei Dumitru", "andrei.d@example.com", "0744 120 557"},
	{"Grace Okafor", "grace.okafor@example.com", ""},
	{"Elena Stan", "elena.stan@example.com", "0733 902 114"},
	{"Tom Hardwick", "tom.hardwick@example.com", ""},
	{"Ioana Marin", "ioana.marin@example.com", "0721 655 803"},
	{"David Chen", "david.chen@example.com", ""},
	{"Maria Constantin", "maria.c@example.com", "0745 338 201"},
	{"Liam O'Brien", "liam.obrien@example.com", ""},
	{"Alexandra Radu", "alex.radu@example.com", "0726 117 940"},
	{"Noah Fischer", "noah.f@example.com", ""},
	{"Cristina Barbu", "cristina.barbu@example.com", "0752 003 476"},
	{"Priya Nair", "priya.nair@example.com", ""},
	{"Vlad Georgescu", "vlad.g@example.com", "0728 845 312"},
}
