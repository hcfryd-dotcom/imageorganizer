// Translation of the UI, applied to the page as it is drawn: text and
// tooltips are written in English in index.html and app.js, and this turns
// them into the chosen language without touching the app's logic. Strings
// built with numbers and names are matched by patterns. The language is a
// per-browser choice (Settings), Danish by default.
(function () {
  'use strict';
  const KEY = 'lang';
  let lang = 'da';
  try { lang = localStorage.getItem(KEY) || 'da'; } catch (e) {}

  // Exact strings (compared trimmed, with runs of spaces as one).
  const DA = {
    // Navigation and headings
    'Photos': 'Billeder', 'Favorites': 'Favoritter', 'Discover': 'Opdag', 'People': 'Personer', 'Family': 'Familie',
    'Find face': 'Find ansigt', 'Map': 'Kort', 'Table': 'Tabel', 'Folders': 'Mapper', 'Review': 'Gennemse',
    'Settings': 'Indstillinger', 'Match': 'Match', 'Library': 'Bibliotek', 'Library folders': 'Biblioteksmapper',
    'Settings — something needs attention': 'Indstillinger — noget kræver opmærksomhed',
    // Status bar
    'Scanning folders': 'Gennemsøger mapper', 'Processing photos': 'Behandler billeder', 'Analysing faces': 'Analyserer ansigter',
    'Server disconnected — reconnecting…': 'Forbindelsen til serveren er afbrudt — forbinder igen…',
    'Connecting to server…': 'Forbinder til serveren…', 'Live · dev': 'Live · udvikling', 'Live': 'Live', 'Offline': 'Offline',
    'Paused': 'Sat på pause', 'Source unreachable — will resume': 'Kilden kan ikke nås — fortsætter senere',
    'Search server offline — will resume': 'Søgeserveren er offline — fortsætter senere',
    'Age and gender, from the face crops': 'Alder og køn ud fra ansigtsudsnittene', 'Age, from the face crops': 'Alder ud fra ansigtsudsnittene', 'about a minute': 'omkring et minut',
    'Faces identified as someone: the ones you confirmed, and the ones filed by themselves because they’re near certain. The rest: no one identified yet (strangers too).':
      'Ansigter, der er genkendt som nogen: dem du har bekræftet, og dem der er arkiveret automatisk, fordi de er næsten sikre. Resten: ingen genkendt endnu (også fremmede).',
    // Photo grid and search
    'Most relevant first': 'Mest relevante først', 'Newest first': 'Nyeste først', 'Oldest first': 'Ældste først',
    'Recently added': 'Senest tilføjet', 'Rating': 'Bedømmelse', 'Largest files': 'Største filer', 'Camera': 'Kamera',
    'Your library is empty': 'Dit bibliotek er tomt',
    'Add a folder and ImageOrganizer will find every photo in it — including subfolders.': 'Tilføj en mappe, så finder ImageOrganizer alle billeder i den — også i undermapper.',
    'Add a folder': 'Tilføj en mappe', 'Photos of': 'Billeder af', 'Show all photos': 'Vis alle billeder', '✦ Smart search': '✦ Smart søgning',
    'Most relevant to “': 'Mest relevante for “', '” first': '” først', 'matching “': 'der matcher “', 'from': 'fra', 'in favorites': 'i favoritter',
    'and…': 'og…', 'Search your photos — “Anna in the garden 2010”, “snow”, “birthday cake”…': 'Søg i dine billeder — “Anna i haven 2010”, “sne”, “fødselsdagskage”…',
    'The 300 best matches, in this order': 'De 300 bedste match, i denne rækkefølge', 'Thumbnail size': 'Miniaturestørrelse', 'Size of the faces': 'Ansigternes størrelse',
    'Ranked by EmbeddingGemma 2 image–text similarity': 'Sorteret efter billede–tekst-lighed (EmbeddingGemma 2)',
    'That’s everything': 'Det var alt', 'Unknown date': 'Ukendt dato', 'Undated': 'Uden dato',
    // Person page
    'All people': 'Alle personer', '· at': '· som', 'looks': 'ligner', 'Birth year': 'Fødselsår', 'Gender': 'Køn',
    'Woman': 'Kvinde', 'Man': 'Mand', 'Not set': 'Ikke angivet', 'unknown': 'ukendt', 'female': 'kvinde', 'woman': 'kvinde', 'man': 'mand',
    'A painting, statue or poster, not a person': 'Et maleri, en statue eller en plakat, ikke en person',
    'Family and friends': 'Familie og venner', 'parent': 'forælder', 'child': 'barn', 'partner': 'partner', 'sibling': 'søskende',
    'half-sibling': 'halvsøskende', 'friend': 'ven', 'Add': 'Tilføj', 'Name': 'Navn', 'Relation': 'Relation', 'Rename': 'Omdøb',
    'Show photos': 'Vis billeder', 'See the photo': 'Se billedet', 'See the photos': 'Se billederne',
    'How old the face reader thinks they look, from the most recent photos. Matching compares new faces with this, not with the birth year.':
      'Hvor gammel ansigtslæseren synes personen ser ud ud fra de nyeste billeder. Nye ansigter sammenlignes med dette, ikke med fødselsåret.',
    'Helps tell relatives apart (mother and son), and names relations (“mother”). Faces of adults are compared with it; children’s aren’t.':
      'Hjælper med at skelne slægtninge (mor og søn) og navngive relationer (“mor”). Voksnes ansigter sammenlignes med det; børns gør ikke.',
    'Never ages, and doesn\'t count as someone in the photo ($alone, $known)': 'Bliver aldrig ældre og tæller ikke som en person på billedet ($alone, $known)',
    'Follows from the relations entered (siblings share parents, a parent’s parent is a grandparent); change those to change this':
      'Følger af de indtastede relationer (søskende har fælles forældre, en forælders forælder er en bedsteforælder); ret dem for at ændre dette',
    'Copy to folder…': 'Kopiér til mappe…', 'Copy their photos into a folder, one subfolder per year': 'Kopiér personens billeder til en mappe med én undermappe pr. år',
    'Copy': 'Kopiér', '’s photos to': 's billeder til', 'Close': 'Luk', 'Stop': 'Stop',
    'The whole path of a folder, e.g. C:\\Users\\you\\Pictures\\Anna': 'Hele stien til en mappe, fx C:\\Users\\dig\\Billeder\\Anna',
    'Only photos you confirmed (leave out ones filed automatically)': 'Kun billeder du har bekræftet (udelad dem, der er arkiveret automatisk)',
    'Each year gets its own folder (2015, 2016 …; photos without a date go in “Unknown year”). The photos are copied; your library isn’t changed. Copying again only adds what’s new.':
      'Hvert år får sin egen mappe (2015, 2016 …; billeder uden dato kommer i “Unknown year”). Billederne kopieres; dit bibliotek ændres ikke. Kopierer du igen, tilføjes kun det nye.',
    'Look': 'Se', 'those that stand out, confirmed in batches first': 'dem der skiller sig ud, bekræftet i bunker først',
    'Their faces that stand out: too unlike them to be filed by itself, more like someone else, the wrong age for the photo, or less like them than when you confirmed it. Faces you confirmed one by one aren’t asked about. Solid border: you confirmed it. Click a face to see its photo.':
      'Deres ansigter, der skiller sig ud: for ulige til at blive arkiveret automatisk, mere lig en anden, forkert alder til billedet eller mindre lig end da du bekræftede det. Ansigter du har bekræftet enkeltvis spørges der ikke om. Fuld kant: du har bekræftet det. Klik på et ansigt for at se billedet.',
    'One face at a time, wherever it was grouped. Dashed: filed automatically, not confirmed yet. “With …”: found because their usual company (family, say) was photographed that day — that helps with small children, who look little like their older selves. Click a face to see its photo.':
      'Ét ansigt ad gangen, uanset hvor det er grupperet. Stiplet: arkiveret automatisk, endnu ikke bekræftet. “Med …”: fundet fordi deres sædvanlige selskab (fx familien) blev fotograferet den dag — det hjælper med små børn, der ligner deres ældre jeg meget lidt. Klik på et ansigt for at se billedet.',
    'Every face of': 'Alle ansigter af',
    ', the newest photos first: the ones you confirmed, and outlined dashed, the ones filed by themselves. ✕ takes a face out; ☆ makes it their picture.':
      ', nyeste billeder først: dem du har bekræftet, og med stiplet kant dem der er arkiveret automatisk. ✕ tager et ansigt ud; ☆ gør det til personens billede.',
    'Faces you said are a picture of': 'Ansigter du har sagt er et billede af',
    '(on a wall, in a frame): theirs, but not them being there. ✕ if one isn’t a picture after all.': '(på en væg, i en ramme): deres, men ikke dem selv til stede. ✕ hvis et alligevel ikke er et billede.',
    'Same person as': 'Samme person som', 'choose…': 'vælg…', 'Merge': 'Slå sammen',
    'Filed by itself: not confirmed': 'Arkiveret automatisk: ikke bekræftet', 'Confirmed': 'Bekræftet', 'Use as picture': 'Brug som billede',
    'Not this person — take out': 'Ikke denne person — tag ud', 'Not a picture: them, for real': 'Ikke et billede: dem selv, i virkeligheden',
    // People list
    'Most photos': 'Flest billeder', 'Fewest photos': 'Færrest billeder', 'Youngest first': 'Yngste først', 'Filter…': 'Filtrér…',
    'Type anywhere on this page to filter; Enter opens the only match': 'Skriv hvor som helst på siden for at filtrere; Enter åbner det eneste match',
    'People are added from your photos: open': 'Personer tilføjes fra dine billeder: åbn', 'and name the faces you know.': 'og navngiv de ansigter, du kender.',
    'Parents, partners, brothers and sisters as entered. Grandparents, aunts and uncles, cousins, step-family and in-laws follow from those. Point at someone to see what everyone else is to them; click to open them.':
      'Forældre, partnere, brødre og søstre som indtastet. Bedsteforældre, tanter og onkler, fætre og kusiner, papfamilie og svigerfamilie følger af dem. Peg på en person for at se, hvad alle andre er for vedkommende; klik for at åbne.',
    'No family entered yet. Add relations on a person’s page, or answer “How are they related?” in Discover.': 'Ingen familie indtastet endnu. Tilføj relationer på en persons side, eller svar på “Hvordan er de i familie?” i Opdag.',
    // Discover
    'Age': 'Alder', 'Min. match': 'Min. match', 'Order': 'Rækkefølge', 'questions': 'spørgsmål', 'Finding questions…': 'Finder spørgsmål…',
    'Questions that teach the app the most come first: “worth” is about how many faces an answer names. Only faces you see get named: untick any that aren’t them, and the rest are confirmed. Faces that are then near certain someone’s are filed under them by themselves (shown as not confirmed), and the next questions get better.':
      'Spørgsmål, der lærer programmet mest, kommer først: “værdi” handler om, hvor mange ansigter et svar navngiver. Kun ansigter du ser bliver navngivet: fjern fluebenet ved dem, der ikke er personen, så bliver resten bekræftet. Ansigter, der derefter næsten sikkert er en bestemt persons, arkiveres automatisk (vist som ikke bekræftet), og de næste spørgsmål bliver bedre.',
    'Learning from your answers — the next questions come in a moment.': 'Lærer af dine svar — de næste spørgsmål kommer om et øjeblik.',
    'Nothing to ask right now. Name someone by clicking their face in a photo, and Discover takes it from there.': 'Intet at spørge om lige nu. Navngiv nogen ved at klikke på deres ansigt i et billede, så tager Opdag den derfra.',
    'Who is this?': 'Hvem er det?', 'A painting or statue': 'Et maleri eller en statue', 'Not a person': 'Ikke en person', 'Stranger': 'Fremmed',
    'Skip': 'Spring over', 'First': 'Først', 'Look closer': 'Se nærmere', 'Save': 'Gem', 'Not sure': 'Ikke sikker', 'A picture': 'Et billede',
    'Same person?': 'Samme person?', 'Two people': 'To personer', 'Date is right': 'Datoen er rigtig', 'Show more questions': 'Vis flere spørgsmål',
    'All': 'Alle', 'More of…': 'Mere af…', 'Related?': 'I familie?', 'Is this really them?': 'Er det virkelig dem?', 'Pictures': 'Billeder på væggen',
    'Dates': 'Datoer', 'Born?': 'Født?', 'Most worth knowing': 'Mest værd at vide', 'Most days with someone you know': 'Flest dage med en du kender',
    'Seen on the most days': 'Set på flest dage', 'In the most photos': 'På flest billeder', 'Seen most recently': 'Set senest',
    'Only faces looking this old in their photo (“Who is this?”: in the last photos of them). Faces whose age can’t be read stay.':
      'Kun ansigter, der ser så gamle ud på billedet (“Hvem er det?”: på de seneste billeder af dem). Ansigter, hvis alder ikke kan aflæses, bliver.',
    'Faces offered (in “More of …?”) and names suggested must be at least this sure': 'Ansigter der tilbydes (i “Mere af …?”) og navne der foreslås skal være mindst så sikre',
    'Click to untick: not them': 'Klik for at fjerne fluebenet: ikke dem', 'Open the photo': 'Åbn billedet',
    'A painting, statue or poster: named, but it never ages and doesn’t count as someone being there': 'Et maleri, en statue eller en plakat: navngivet, men bliver aldrig ældre og tæller ikke som en person til stede',
    'Someone you don’t know: hidden everywhere': 'En du ikke kender: skjult overalt', 'Don’t ask again': 'Spørg ikke igen',
    'Open a photo of the two of them': 'Åbn et billede af de to', 'A photo of a photo: on a wall, in an album': 'Et billede af et billede: på en væg, i et album',
    'They did swap places: both are right': 'De byttede faktisk plads: begge er rigtige',
    'The middle year of the session: closer than the day they were copied, though not exact': 'Det midterste år for serien: tættere på end den dag de blev kopieret, men ikke præcist',
    'Year born': 'Fødselsår', 'Other year': 'Andet år',
    // Lane match
    'Lane Match': 'Rækkematch', 'experiment': 'eksperiment', 'File near-certain faces by themselves': 'Arkivér næsten sikre ansigter automatisk',
    'Each row is a person; to the right, the faces likeliest theirs, best first. Click a face if it’s them, ✕ if not. A few seconds after your last answer the rows refresh with what was learnt. With “File near-certain faces by themselves” on, faces now near certain are filed too (as not confirmed; undo in the row).':
      'Hver række er en person; til højre de ansigter, der mest sandsynligt er deres, bedste først. Klik på et ansigt, hvis det er dem, ✕ hvis ikke. Et par sekunder efter dit sidste svar opdateres rækkerne med det, der er lært. Med “Arkivér næsten sikre ansigter automatisk” slået til arkiveres ansigter, der nu er næsten sikre, også (som ikke bekræftet; fortryd i rækken).',
    'After a pause in answering, faces very alike to someone (70% or more, clearly more than to anyone else, age and gender fitting) are filed under them as not confirmed; undo in the row':
      'Efter en pause i svarene arkiveres ansigter, der ligner en person meget (70 % eller mere, tydeligt mere end nogen anden, med passende alder og køn), under personen som ikke bekræftet; fortryd i rækken',
    'Undo': 'Fortryd', 'A picture on the wall?': 'Et billede på væggen?', 'Nothing likely left for them right now.': 'Intet sandsynligt tilbage for dem lige nu.',
    'Add a person…': 'Tilføj en person…', 'Close this row': 'Luk denne række', 'Not them': 'Ikke dem',
    // Find face
    'Looking…': 'Leder…',
    'Paste a picture (Ctrl+V), drop one anywhere, or choose one. Its faces are looked for among yours, so you can name or confirm them. The picture is only looked at; it isn’t kept or added to your library.':
      'Indsæt et billede (Ctrl+V), slip et hvor som helst, eller vælg et. Dets ansigter søges blandt dine, så du kan navngive eller bekræfte dem. Billedet bliver kun set på; det gemmes ikke og føjes ikke til dit bibliotek.',
    'Paste, drop or': 'Indsæt, slip eller', 'choose a picture': 'vælg et billede', 'No one alike in your photos.': 'Ingen der ligner på dine billeder.',
    'Faces in it: pick one': 'Ansigter i det: vælg et', 'The face': 'Ansigtet', 'Not named yet': 'Ikke navngivet endnu', 'A stranger (hidden)': 'En fremmed (skjult)',
    'Correct': 'Korrekt', 'They are': 'Det er', 'Clear': 'Ryd', 'Who is this group?': 'Hvem er denne gruppe?', 'New or existing person': 'Ny eller eksisterende person',
    'Click to tick; the magnifier opens the photo': 'Klik for at sætte flueben; forstørrelsesglasset åbner billedet',
    'Tick these': 'Sæt flueben ved disse', 'Untick these': 'Fjern flueben ved disse', 'That’s not a picture': 'Det er ikke et billede',
    'No face found in that picture.': 'Der blev ikke fundet noget ansigt i billedet.',
    // Match
    'Looks': 'Ser ud som', 'to': 'til', 'any': 'alle', 'Best matches first': 'Bedste match først', 'Newest photos first': 'Nyeste billeder først',
    'Oldest photos first': 'Ældste billeder først',
    'How old the unknown face looks. With an age set, faces too small to judge are left out.': 'Hvor gammelt det ukendte ansigt ser ud. Med en alder angivet udelades ansigter, der er for små til at vurdere.',
    'Unknown faces, likeliest matches first. On top, a face you confirmed of the person it most likely is; below, the unknown face. Same person? Y, N and S answer the first card. Click a face to see its photo.':
      'Ukendte ansigter, mest sandsynlige match først. Øverst et ansigt du har bekræftet af den person, det mest sandsynligt er; nedenunder det ukendte ansigt. Samme person? Y, N og S besvarer det første kort. Klik på et ansigt for at se billedet.',
    'Is this the same person?': 'Er det den samme person?', 'Yes': 'Ja', 'No': 'Nej',
    'No likely matches left. Name a few more people, and more will turn up.': 'Ingen sandsynlige match tilbage. Navngiv nogle flere personer, så dukker der flere op.',
    'Click to see the photo': 'Klik for at se billedet', 'Decide later (S)': 'Beslut senere (S)', 'Decide later': 'Beslut senere',
    // Review
    'Largest groups first': 'Største grupper først', 'Likely match first': 'Sandsynligt match først', 'People you probably know': 'Personer du sikkert kender',
    'Each card is one person at one age, found in your photos. Pick who it is, type a new name, or skip. Photos that clearly match someone you’ve named are filed automatically and never show up here.':
      'Hvert kort er én person i én alder, fundet i dine billeder. Vælg hvem det er, skriv et nyt navn, eller spring over. Billeder, der tydeligt matcher en du har navngivet, arkiveres automatisk og dukker aldrig op her.',
    'File every group that is at least': 'Arkivér alle grupper, der er mindst', '% like someone you’ve named': '% lig en du har navngivet',
    'Minimum likeness in percent': 'Mindste lighed i procent', 'GO': 'GO', 'Filing…': 'Arkiverer…',
    'No group is that alike to anyone yet. Name a few more, or lower the percentage.': 'Ingen gruppe ligner nogen så meget endnu. Navngiv nogle flere, eller sænk procenten.',
    '· Cards marked': '· Kort markeret med', '✓ sure': '✓ sikker', 'below are among them.': 'nedenfor er blandt dem.',
    'Show who they’d be filed as': 'Vis hvem de ville blive arkiveret som', 'Hide who they’d be filed as': 'Skjul hvem de ville blive arkiveret som',
    'Lower percentages file more, with more mistakes. Filed groups can be checked on each person’s page and moved if wrong.':
      'Lavere procent arkiverer flere, med flere fejl. Arkiverede grupper kan tjekkes på hver persons side og flyttes, hvis de er forkerte.',
    'From your photo': 'Fra dit billede', 'Back to inbox': 'Tilbage til indbakken', 'Filed as': 'Arkiveret som', 'automatically': 'automatisk',
    'Skip for now': 'Spring over indtil videre', 'All caught up — nothing to review.': 'Alt er gennemgået — intet at gennemse.',
    'You confirmed this one': 'Du har bekræftet dette', 'Looks least like the rest — check this one': 'Ligner resten mindst — tjek dette',
    'Open the photo to confirm just this face': 'Åbn billedet for kun at bekræfte dette ansigt',
    'Photographed together with people you’ve named: on how many days, over which years, in how many places': 'Fotograferet sammen med personer du har navngivet: på hvor mange dage, over hvilke år, på hvor mange steder',
    'Someone you don’t know: hidden in all their photos, and never suggested': 'En du ikke kender: skjult på alle deres billeder og aldrig foreslået',
    'Not faces at all: a statue, a pattern, a pet…': 'Slet ikke ansigter: en statue, et mønster, et kæledyr…',
    // Settings
    'Search by meaning': 'Søgning efter indhold',
    'A llama.cpp server running a multimodal embedding model (EmbeddingGemma 2 with': 'En llama.cpp-server med en multimodal embedding-model (EmbeddingGemma 2 med',
    '). It turns photos and search text into comparable vectors.': '). Den gør billeder og søgetekst til sammenlignelige vektorer.',
    'Embedding server': 'Embedding-server', 'URL of llama-server, or “off” to turn search by meaning off. Applies at once.': 'Adressen på llama-server, eller “off” for at slå søgning efter indhold fra. Gælder med det samme.',
    'Test': 'Test', 'Model': 'Model', 'Whatever the server has loaded': 'Den model serveren har indlæst',
    'Sent as “model” with each request, for servers that host several. Photos are re-indexed when the model changes.': 'Sendes som “model” med hver forespørgsel til servere med flere modeller. Billeder indekseres igen, når modellen skifter.',
    'Parallel requests': 'Samtidige forespørgsler', 'Match llama-server’s': 'Sæt som llama-servers', '. After restart.': '. Efter genstart.',
    'Faces': 'Ansigter', 'Detection, recognition and age models run with ONNX Runtime. These apply after a restart.': 'Modeller til at finde, genkende og aldersbestemme ansigter kører med ONNX Runtime. Gælder efter en genstart.',
    'Models folder': 'Mappe med modeller', 'ONNX Runtime library': 'ONNX Runtime-bibliotek',
    'A GPU build of ONNX Runtime lets faces run on the graphics card.': 'En GPU-udgave af ONNX Runtime lader ansigtsanalysen køre på grafikkortet.',
    'Run on': 'Kør på', 'Automatic: GPU if available': 'Automatisk: GPU hvis tilgængelig', 'GPU (CUDA)': 'GPU (CUDA)', 'CPU': 'CPU',
    'GPU memory per model': 'GPU-hukommelse pr. model', 'MB': 'MB', 'Sensitive photos': 'Følsomme billeder', 'Show photos marked NSFW': 'Vis billeder markeret som NSFW',
    'Photos with nudity, as judged by a classifier from their thumbnails (or marked by you in the photo viewer), are left out of browsing, search and the map. Search for':
      'Billeder med nøgenhed, vurderet af en klassifikator ud fra miniaturerne (eller markeret af dig i billedfremviseren), udelades fra gennemsyn, søgning og kortet. Søg efter',
    'to see just those.': 'for kun at se dem.', 'File confident matches automatically': 'Arkivér sikre match automatisk',
    'Groups that look very much like someone you confirmed are filed under them, and wait in Review for you to confirm.': 'Grupper, der ligner en du har bekræftet meget, arkiveres under personen og venter i Gennemse på din bekræftelse.',
    'Confidence needed': 'Nødvendig sikkerhed', '60% was always right in tests on a real library; lower files more, with more mistakes.': '60 % var altid rigtigt i test på et rigtigt bibliotek; lavere arkiverer flere, med flere fejl.',
    'Smallest group in Review': 'Mindste gruppe i Gennemse', 'Groups with fewer photos are left out of Review.': 'Grupper med færre billeder udelades fra Gennemse.',
    'Import': 'Import', 'These apply after a restart.': 'Gælder efter en genstart.', 'Parallel imports': 'Samtidige importer',
    'Connections per network folder': 'Forbindelser pr. netværksmappe', 'Look for new photos every': 'Søg efter nye billeder hvert', 'minutes (−1: never)': 'minut (−1: aldrig)',
    'Map tiles': 'Kortfliser', 'A tile server URL with {z}, {x} and {y}. Applies when the map is next opened.': 'Adressen på en kortflise-server med {z}, {x} og {y}. Gælder næste gang kortet åbnes.',
    'Greyed-out settings are set on the command line (': 'Gråtonede indstillinger er sat på kommandolinjen (', '), which takes priority.': '), som har forrang.',
    'Data folder:': 'Datamappe:', 'Undo changes': 'Fortryd ændringer', 'Saved — some changes need a restart': 'Gemt — nogle ændringer kræver genstart',
    'Saved': 'Gemt', 'No server set': 'Ingen server angivet', 'Restarting…': 'Genstarter…', 'Restart now': 'Genstart nu', 'Language': 'Sprog',
    // Photo viewer
    '← Back to photo': '← Tilbage til billedet', 'Taken': 'Taget', 'Dimensions': 'Mål', 'Size': 'Størrelse', 'Exposure': 'Eksponering', 'Location': 'Sted',
    '← Back to results': '← Tilbage til resultaterne', 'Looks blurry': 'Ser sløret ud', 'Not NSFW': 'Ikke NSFW', 'Mark as NSFW': 'Markér som NSFW',
    'Reset': 'Nulstil', 'Date set by you; the file says': 'Dato sat af dig; filen siger', 'Use the file’s date': 'Brug filens dato', 'Show on map': 'Vis på kort',
    'Locations': 'Steder', 'Also on the same wall:': 'Også på samme væg:', 'someone not named': 'en der ikke er navngivet', 'Not a picture': 'Ikke et billede',
    'Yes, mark the ticked ones as pictures': 'Ja, markér dem med flueben som billeder', 'Strangers at this event': 'Fremmede ved denne begivenhed',
    'Is any of these someone you know?': 'Er nogen af disse en du kender?',
    'They look like people you’ve named. Yes names them; no hides them with the rest; unanswered ones are left alone.': 'De ligner personer du har navngivet. Ja navngiver dem; nej skjuler dem sammen med resten; ubesvarede lades være.',
    'Click a photo to leave it out (a family photo that got caught in); the magnifier opens it. The number is how many unnamed faces it has.':
      'Klik på et billede for at udelade det (et familiebillede, der kom med ved en fejl); forstørrelsesglasset åbner det. Tallet er, hvor mange unavngivne ansigter det har.',
    'Turn left — for photos the camera saved sideways (Shift+R)': 'Drej til venstre — til billeder kameraet har gemt på siden (Shift+R)',
    'Turn right (R)': 'Drej til højre (R)', 'Show who is in the photo (F)': 'Vis hvem der er på billedet (F)', 'Favorite (H)': 'Favorit (H)',
    'Download original': 'Hent originalen', 'Photos taken around the same time (T)': 'Billeder taget omkring samme tid (T)', 'Info (I)': 'Info (I)',
    'Little fine detail even in its sharpest parts; search $blurry for all such photos': 'Kun lidt fine detaljer selv i de skarpeste dele; søg efter $blurry for alle sådanne billeder',
    'Left out of browsing and search, unless shown in Settings': 'Udeladt fra gennemsyn og søgning, medmindre det er slået til i Indstillinger',
    'Leave it to the classifier': 'Overlad det til klassifikatoren', 'Wrong date? Set the right one': 'Forkert dato? Angiv den rigtige',
    'Add a title': 'Tilføj en titel', 'Notes': 'Noter', 'Who it is a picture of': 'Hvem det er et billede af', 'Close (Esc)': 'Luk (Esc)', 'Done (Esc)': 'Færdig (Esc)',
    'Someone else…': 'En anden…', 'Someone you don’t know: hides them in all their photos, and they’re never suggested': 'En du ikke kender: skjuler dem på alle deres billeder, og de foreslås aldrig',
    'A cat, a statue, a pattern… — not a face at all': 'En kat, en statue, et mønster… — slet ikke et ansigt', 'also in this photo': 'også på dette billede',
    'Click to leave out': 'Klik for at udelade', 'Left out: click to include': 'Udeladt: klik for at tage med',
    // Folders
    'Folders can be local or on a network share. ImageOrganizer scans them and all their subfolders; moving files around keeps every rating, note and person tag.':
      'Mapper kan være lokale eller på et netværksdrev. ImageOrganizer gennemsøger dem og alle deres undermapper; flytter du filer rundt, bevares alle bedømmelser, noter og personmarkeringer.',
    'Remove': 'Fjern', 'No folders yet.': 'Ingen mapper endnu.', 'This computer': 'Denne computer', 'Network share (SMB)': 'Netværksdrev (SMB)', 'Browse…': 'Gennemse…',
    'photos': 'billeder', 'on disk': 'på disken', 'geotagged': 'med geotag', 'duplicate files': 'dublerede filer', 'missing (kept)': 'mangler (bevaret)',
    'No subfolders': 'Ingen undermapper', 'Cancel': 'Annullér', 'Choose this folder': 'Vælg denne mappe', 'Up': 'Op', 'Password': 'Adgangskode',
    'Server, e.g. nas.local': 'Server, fx nas.local', 'User (DOMAIN;user ok)': 'Bruger (DOMÆNE;bruger er ok)', '/home/you/Pictures': 'C:\\Users\\dig\\Billeder',
    'Folder added — scanning': 'Mappe tilføjet — gennemsøger', 'Rescan started': 'Ny gennemsøgning startet', 'Rescan': 'Gennemsøg igen', 'Scan': 'Gennemsøg',
    'Oldest photo first. Hover a face: ★ makes it the picture, ✕ takes it out. Click to see the photo.': 'Ældste billede først. Hold musen over et ansigt: ★ gør det til billedet, ✕ tager det ud. Klik for at se billedet.',
    // Notifications
    'Noted: two people': 'Noteret: to personer', 'Kept as entered': 'Bevaret som indtastet', 'Won’t ask again': 'Spørger ikke igen', 'Kept its date': 'Beholdt sin dato',
    'Marked as a picture on the wall': 'Markeret som et billede på væggen', 'Noted: not a picture': 'Noteret: ikke et billede',
    'Turned — finding faces again…': 'Drejet — finder ansigter igen…', 'No longer marked as a picture': 'Ikke længere markeret som et billede',
    'Back to the file’s date': 'Tilbage til filens dato', 'Date saved — the file itself is unchanged': 'Dato gemt — selve filen er uændret',
    'Hidden as a stranger': 'Skjult som fremmed', 'Removed — it won’t be detected as a person again': 'Fjernet — det bliver ikke fundet som en person igen',
    'Picture updated': 'Billede opdateret', 'Put back': 'Lagt tilbage', 'Update ready — it will apply when you finish typing': 'Opdatering klar — den træder i kraft, når du er færdig med at skrive',
    'Styles updated': 'Udseende opdateret', 'Server updated': 'Server opdateret', 'not named yet': 'ikke navngivet endnu', 'filed but not confirmed': 'arkiveret men ikke bekræftet',
    'exactly one person': 'præcis én person', 'two or more people': 'to eller flere personer', 'one person only': 'kun én person', 'not safe for work': 'ikke egnet til arbejdet',
    'a picture of someone': 'et billede af nogen', 'only one person in the photo': 'kun én person på billedet', 'photos marked not safe for work': 'billeder markeret som ikke egnet til arbejdet',
    'photos that look blurry': 'billeder der ser slørede ud',
    'a picture of someone in it (on a wall, in a frame); with a name: pictures of them': 'et billede af nogen i det (på en væg, i en ramme); med et navn: billeder af dem',
    'Unknown year': 'Ukendt år',
    'Add folder': 'Tilføj mappe',
    'Connecting…': 'Forbinder…',
    'Saving…': 'Gemmer…',
    'Testing…': 'Tester…',
    'Scanning…': 'Gennemsøger…',
    'Rescan now': 'Gennemsøg nu',
    'Hiding…': 'Skjuler…',
    'Unknown': 'Ukendt',
    'Untick all': 'Fjern alle flueben',
    'Tick all': 'Sæt flueben ved alle',
    'As entered': 'Som indtastet',
    'Neither': 'Ingen af delene',
    'nothing': 'intet',
    'Not NSFW (your call)': 'Ikke NSFW (dit valg)',
    'Taken around the same time': 'Taget omkring samme tid',
    'No date: the photos next to it in its folder': 'Ingen dato: billederne ved siden af i samme mappe',
    'Don’t know anyone at this event?': 'Kender du ingen ved denne begivenhed?',
    'Unknown camera': 'Ukendt kamera',
    'not scanned yet': 'ikke gennemsøgt endnu',
    'Group': 'Gruppe',
    'Search by meaning is off: no embedding server is set.': 'Søgning efter indhold er slået fra: der er ikke angivet nogen embedding-server.',
    'step-parent': 'stedforælder',
    'step-sibling': 'stedsøskende',
    'step-child': 'stedbarn',
    'grandparent': 'bedsteforælder',
    'child': 'barn',
    'parent': 'forælder',
    'sibling': 'søskende',
    'Knowing it, every face of theirs is judged by their real age.': 'Når det er kendt, bedømmes alle deres ansigter ud fra deres rigtige alder.',
    'A painting, statue or poster: hidden everywhere. To keep it by name, type the name of the painting instead.': 'Et maleri, en statue eller en plakat: skjult overalt. For at beholde det med navn, skriv i stedet maleriets navn.',
    'A painting, statue or poster: hidden everywhere (the very same face for years: likely). To keep it by name, type the name of the painting instead.': 'Et maleri, en statue eller en plakat: skjult overalt (præcis samme ansigt i årevis: sandsynligt). For at beholde det med navn, skriv i stedet maleriets navn.',
    'Marked NSFW by you': 'Markeret som NSFW af dig',
    'Within each year': 'I hvert år',
    'no subfolders': 'ingen undermapper',
    'a folder per area, from the photo’s GPS position (municipality, e.g. “Frederikshavn”)': 'en mappe pr. område ud fra billedets GPS-position (kommune, fx “Frederikshavn”)',
    'a folder per town, from the photo’s GPS position (nearest town, e.g. “Skagen”)': 'en mappe pr. by ud fra billedets GPS-position (nærmeste by, fx “Skagen”)',
    'Each year gets its own folder (2015, 2016 …; photos without a date go in “Unknown year”). With places, photos without a GPS position stay in the year’s folder, and places abroad get their country (“Barcelona, Spanien”). The photos are copied; your library isn’t changed. Copying again only adds what’s new.': 'Hvert år får sin egen mappe (2015, 2016 …; billeder uden dato kommer i “Unknown year”). Med steder bliver billeder uden GPS-position liggende i årets mappe, og steder i udlandet får landet med (“Barcelona, Spanien”). Billederne kopieres; dit bibliotek ændres ikke. Kopierer du igen, tilføjes kun det nye.',
    'Delete: move to the deleted-photos folder (Del)': 'Slet: flyt til mappen med slettede billeder (Del)',
    'Delete this photo?': 'Slet dette billede?',
    'It is moved, not erased, to this folder (into a subfolder named like its library folder), so you can get it back. The library keeps its names and ratings, should you move it back.': 'Det flyttes, ikke slettes helt, til denne mappe (i en undermappe med samme navn som biblioteksmappen), så du kan hente det tilbage. Biblioteket husker navne og bedømmelser, hvis du flytter det tilbage.',
    'Deleted-photos folder': 'Mappe til slettede billeder',
    'Moving…': 'Flytter…',
    'Delete': 'Slet',
    // Relations
    'aunt or uncle': 'tante eller onkel',
    'aunt': 'tante',
    'brother': 'bror',
    'brother-in-law': 'svoger',
    'daughter': 'datter',
    'daughter-in-law': 'svigerdatter',
    'father': 'far',
    'father-in-law': 'svigerfar',
    'granddaughter': 'barnebarn',
    'grandfather': 'bedstefar',
    'grandmother': 'bedstemor',
    'grandson': 'barnebarn',
    'half-brother': 'halvbror',
    'half-sister': 'halvsøster',
    'mother': 'mor',
    'mother-in-law': 'svigermor',
    'nephew': 'nevø',
    'niece or nephew': 'niece eller nevø',
    'niece': 'niece',
    'sister': 'søster',
    'sister-in-law': 'svigerinde',
    'son': 'søn',
    'son-in-law': 'svigersøn',
    'stepbrother': 'stedbror',
    'stepdaughter': 'steddatter',
    'stepfather': 'stedfar',
    'stepmother': 'stedmor',
    'stepsister': 'stedsøster',
    'stepson': 'stedsøn',
    'uncle': 'onkel',
    'grandchild': 'barnebarn',
    'grandparent': 'bedsteforælder',
    'partners': 'partnere',
    'cousin': 'fætter eller kusine',
    // Places: selecting photos, setting and borrowing their place
    'Select': 'Vælg', 'Done': 'Færdig', 'Select photos, e.g. to set their place': 'Vælg billeder, fx for at sætte deres sted',
    'Select all': 'Vælg alle', 'Deselect': 'Fravælg', 'Click photos to select them': 'Klik på billeder for at vælge dem',
    'Place borrowed from a photo taken around the same time': 'Stedet er lånt fra et billede taget omkring samme tid',
    'Place set by you': 'Stedet er sat af dig', 'Place from the camera': 'Stedet er fra kameraet',
    'Set place…': 'Sæt sted…', 'Remove place': 'Fjern sted', 'Clear selection': 'Ryd valg',
    'Shift-click selects a run of photos': 'Skift-klik vælger alle billeder imellem',
    'More places…': 'Flere steder…', 'Find places for more photos': 'Find steder til flere billeder',
    'A photo without a place can borrow one from a photo taken shortly before or after it, on your phone say. When the photos either side were taken far apart, it gets one only if one of them is clearly nearer in time. Only the library is changed, not your files.':
      'Et billede uden sted kan låne stedet fra et billede taget kort før eller efter, fx med din telefon. Er billederne før og efter taget langt fra hinanden, får det kun et sted, hvis det ene er tydeligt tættere på i tid. Kun biblioteket ændres, ikke dine filer.',
    'Taken within': 'Taget inden for', '1 hour': '1 time', '3 hours': '3 timer', '12 hours': '12 timer', '24 hours': '24 timer',
    'Counting…': 'Tæller…', 'No more photos can borrow a place': 'Der er ikke flere billeder, der kan låne et sted',
    'Give them a place': 'Giv dem et sted', 'Remove them all': 'Fjern dem alle', 'Yes, remove all borrowed places': 'Ja, fjern alle lånte steder',
    'To set the place of many photos yourself: in Photos, click Select.': 'Vil du selv sætte stedet på mange billeder: klik på Vælg under Billeder.',
    'Borrowed from a photo taken around the same time': 'Lånt fra et billede taget omkring samme tid', 'Set by you': 'Sat af dig',
    'Change': 'Ret', 'No place': 'Intet sted', 'Set the place': 'Sæt stedet', 'Search for a town…': 'Søg efter en by…',
    'Search for a town, or click on the map': 'Søg efter en by, eller klik på kortet',
    'Photos whose camera saved a place keep it. Only the library is changed, not your files.':
      'Billeder, hvor kameraet har gemt et sted, beholder det. Kun biblioteket ændres, ikke dine filer.',
    // Folders: the whole library into one tidy folder
    'Collect everything in one tidy folder': 'Saml alt i én ryddet mappe',
    'Copies every photo from the folders above into one folder, sorted by year, month and what took it (iPhone, Canon, Screenshots …). Each photo comes once: copies of the same photo are left out. Your folders aren’t changed, and copying again only adds what’s new.':
      'Kopierer alle billeder fra mapperne ovenfor til én mappe, sorteret efter år, måned og hvad der tog dem (iPhone, Canon, Skærmbilleder …). Hvert billede kommer kun med én gang: kopier af det samme billede springes over. Dine mapper ændres ikke, og kopierer du igen, tilføjes kun det nye.',
    'Next: look through the new folder. When you’re happy with it, add it above and remove the old folders: the photos are recognised, so names and places stay. The old folders are left for you to delete or keep.':
      'Næste skridt: kig den nye mappe igennem. Når du er tilfreds, tilføjer du den ovenfor og fjerner de gamle mapper: billederne bliver genkendt, så navne og steder bliver. De gamle mapper lader programmet være, så du selv kan slette eller gemme dem.',
    'Not in photos before…': 'Ikke på billeder før…', 'Write the date as day-month-year, e.g. 10-11-2008': 'Skriv datoen som dag-måned-år, fx 10-11-2008',
    'Take this person out of every photo taken before a date, such as their birth: those faces become one group to name in Review': 'Fjern personen fra alle billeder taget før en dato, fx fødselsdagen: de ansigter samles i én gruppe under Gennemse, så du kan navngive dem',
    // Slideshow and photo book
    'Slideshow: one photo after another, full screen': 'Diasshow: ét billede ad gangen, i fuld skærm', 'Stop the slideshow (Esc)': 'Stop diasshowet (Esc)',
    'Make photo book': 'Lav fotobog', 'Photo book': 'Fotobog', 'Title': 'Titel', 'Page': 'Side',
    'A4 landscape': 'A4 på langs', 'A4 portrait': 'A4 på højkant', 'Square (21 × 21 cm)': 'Kvadratisk (21 × 21 cm)',
    'Photos per page': 'Billeder pr. side', 'As they fit best': 'Som de passer bedst', 'Dates under the photos': 'Datoer under billederne',
    'Print / save as PDF': 'Udskriv / gem som PDF', 'Loading the photos…': 'Henter billederne…',
    'There are no photos to put in a book here.': 'Der er ingen billeder at lave en bog af her.',
    'In the print window choose “Save as PDF” (or a printer), and under more settings set margins to “None” and turn on background graphics. The PDF can be printed at home or sent to a print shop that takes PDF files.':
      'Vælg “Gem som PDF” (eller en printer) i udskriftsvinduet, og sæt under flere indstillinger margener til “Ingen” og slå baggrundsgrafik til. PDF’en kan printes hjemme eller sendes til et trykkeri, der tager imod PDF-filer.',
    // A picture from a video
    'Save picture': 'Gem billede', 'Name of the picture': 'Navn på billedet',
    'Pause where you want it: the frame is saved as a picture next to the video': 'Sæt på pause, hvor du vil have det: billedet gemmes ved siden af videoen',
    // Albums
    'Albums': 'Album', 'New album': 'Nyt album', 'All albums': 'Alle album', 'Delete album': 'Slet album',
    'Add to album…': 'Tilføj til album…', 'Remove from album': 'Fjern fra album', 'Make album': 'Lav album',
    'Add the photo to an album': 'Tilføj billedet til et album', 'Take it out of this album': 'Tag det ud af dette album',
    'New album, e.g. Summer 2024': 'Nyt album, fx Sommer 2024', 'No albums yet: make the first below.': 'Ingen album endnu: lav det første herunder.',
    'Name of the new album': 'Navn på det nye album', 'New name of the album': 'Nyt navn på albummet',
    'Took 1 photo out of the album': 'Tog 1 billede ud af albummet',
    'This album is empty. In Photos, click Select, choose some photos and then “Add to album…”.':
      'Albummet er tomt. Under Billeder: klik på Vælg, vælg nogle billeder og derefter “Tilføj til album…”.',
    'No albums yet. Make one here, or in Photos: click Select, choose photos and then “Add to album…”. A photo can be in several albums, from any folder, and taking it out of an album never deletes it.':
      'Ingen album endnu. Lav et her, eller under Billeder: klik på Vælg, vælg billeder og derefter “Tilføj til album…”. Et billede kan være i flere album, fra alle mapper, og at tage det ud af et album sletter det aldrig.',
    // Settings: backup of the library
    'Backup of the library': 'Sikkerhedskopi af biblioteket',
    'Once a day the program copies its library (names of faces, places, dates, albums) to a folder of its own. Best on another drive than the program’s. Your photos aren’t copied.':
      'Én gang i døgnet kopierer programmet sit bibliotek (navne på ansigter, steder, datoer, album) til en mappe for sig. Helst på et andet drev end programmets. Dine billeder kopieres ikke.',
    'Folder for the copies': 'Mappe til kopierne', 'Empty: the data folder’s “backups”. “off”: no copies.': 'Tom: datamappens “backups”. “off”: ingen kopier.',
    'Copies to keep': 'Kopier der gemmes', 'days': 'dage', 'Last copy': 'Seneste kopi', 'None yet': 'Ingen endnu', 'Off': 'Slået fra',
    'Back up now': 'Tag en kopi nu', 'Copying…': 'Kopierer…',
    'Fetch new photos': 'Hent nye billeder', 'Copy into main folder': 'Kopiér til hovedmappe', 'Fetched 1 new photo': 'Hentede 1 nyt billede',
    'Copies the photos you don’t have yet from another folder (your phone’s OneDrive camera folder, say) into the tidy folder above, sorted the same way, and takes them into the library. That folder is only read. The first time it reads every file there; after that only new ones.':
      'Kopierer de billeder, du ikke har endnu, fra en anden mappe (fx din telefons OneDrive-mappe) ind i den ryddede mappe ovenfor, sorteret på samme måde, og tager dem ind i biblioteket. Den anden mappe bliver kun læst. Første gang læses alle filer dér; derefter kun de nye.',
    'Place set for 1 photo': 'Sted sat på 1 billede', 'Place removed from 1 photo': 'Sted fjernet fra 1 billede', '1 photo got a place': '1 billede fik et sted',
  };

  const MONTHS = {January: 'januar', February: 'februar', March: 'marts', April: 'april', May: 'maj', June: 'juni', July: 'juli', August: 'august', September: 'september', October: 'oktober', November: 'november', December: 'december'};
  const NUM = '(\\d[\\d.,\\u00a0]*)';
  const n1 = (n, one, many) => (String(n).trim() === '1' ? one : many);

  // Strings with numbers and names in them.
  const PATTERNS = [
    [/^born (~?\d+)$/, (m, y) => 'født ' + y],
    [/^b\. (\d+)$/, (m, y) => 'f. ' + y],
    [/^born ~(\d+)$/, (m, y) => 'født ~' + y],
    [/^Save (\d+)$/, (m, n) => 'Gem ' + n],
    [/^Accept (\d+)$/, (m, n) => 'Godkend ' + n],
    [/^(\d[\d.,\u00a0]*) photos on (\d[\d.,\u00a0]*) days(, .*?)?( · born ~(\d+))?$/, (m, a, b, y, bb, by) => a + ' billeder på ' + b + ' dage' + (y || '') + (by ? ' · født ~' + by : '')],
    [/^Often with (.*) · (\d[\d.,\u00a0]*) days with people you’ve named$/, (m, w, n) => 'Ofte sammen med ' + w.replace(/ and /g, ' og ') + ' · ' + n + ' dage med personer du har navngivet'],
    [/^with (.*) \(([a-z -]+)\) in (\d[\d.,\u00a0]*) of (\d[\d.,\u00a0]*) photos$/, (m, w, r, a, b) => 'med ' + w + ' (' + (DA[r] || r) + ') på ' + a + ' af ' + b + ' billeder'],
    [/^(with|next to) (.*) \(([a-z -]+)\)$/,(m, w, nm, r) => (w === 'with' ? 'med ' : 'ved siden af ') + nm + ' (' + (DA[r] || r) + ')'],
    [/^with (.*)$/, (m, w) => 'med ' + w.replace(/ and /g, ' og ')],
    [/^(.+)’s ([a-z -]+)$/, (m, nm, k) => DA[k] ? nm + 's ' + DA[k] : null],
    [/^More of (.*)\?$/, (m, nm) => 'Mere af ' + nm + '?'],
    [/^When was (.*) born\?$/, (m, nm) => 'Hvornår er ' + nm + ' født?'],
    [/^Both (.*)$/, (m, nm) => 'Begge ' + nm],
    [/^Looks like (.*), but is in photos with them: a relative\?$/, (m, nm) => 'Ligner ' + nm + ', men er på billeder sammen med dem: en slægtning?'],
    [/^Looks most like (.*?)( \((\d+)% alike\))?$/, (m, nm, x, p) => 'Ligner mest ' + nm + (p ? ' (' + p + ' % ens)' : '')],
    [/^only (\d+) confirmed so far$/, (m, n) => 'kun ' + n + ' bekræftet indtil videre'],
    [/^worth ~(.*)$/, (m, n) => 'værdi ~' + n],
    [/^(\d+) of (\d+) ticked$/, (m, a, b) => a + ' af ' + b + ' med flueben'],
    [/^\((\d+) of (\d+) ticked\)$/, (m, a, b) => '(' + a + ' af ' + b + ' med flueben)'],
    [/^(.*) \((\d+) of (\d+) ticked\)$/, (m, nm, a, b) => nm + ' (' + a + ' af ' + b + ' med flueben)'],
    [/^(\d[\d.,\u00a0]*) (faces?) ticked$/, (m, n) => n + ' ' + n1(n, 'ansigt', 'ansigter') + ' med flueben'],
    [/^(\d[\d.,\u00a0]*) (faces?), likeliest first$/, (m, n) => n + ' ' + n1(n, 'ansigt', 'ansigter') + ', mest sandsynlige først'],
    [/^(\d[\d.,\u00a0]*) faces left$/, (m, n) => n + ' ansigter tilbage'],
    [/^(\d[\d.,\u00a0]*) left$/, (m, n) => n + ' tilbage'],
    [/^(\d[\d.,\u00a0]*) found$/, (m, n) => n + ' fundet'],
    [/^(\d[\d.,\u00a0]*) new or changed$/, (m, n) => n + ' nye eller ændrede'],
    [/^(\d[\d.,\u00a0]*) questions$/, (m, n) => n + ' spørgsmål'],
    [/^(\d[\d.,\u00a0]*) named$/, (m, n) => n + ' navngivet'],
    [/^(\d[\d.,\u00a0]*) of (\d[\d.,\u00a0]*)$/, (m, a, b) => a + ' af ' + b],
    [/^(\d[\d.,\u00a0]*) (famil(y|ies))$/, (m, n) => n + ' ' + n1(n, 'familie', 'familier')],
    [/^(\d[\d.,\u00a0]*) to check$/, (m, n) => n + ' at tjekke'],
    [/^✓ (\d+) filed by themselves$/, (m, n) => '✓ ' + n + ' arkiveret automatisk'],
    [/^(\d[\d.,\u00a0]*) photos left out$/, (m, n) => n + ' billeder udeladt'],
    [/^(\d[\d.,\u00a0]*) geotagged photos( matching)?$/, (m, n, x) => n + ' billeder med geotag' + (x ? ' der matcher' : '')],
    [/^(\d[\d.,\u00a0]*) photos? selected$/, (m, n) => n + (n === '1' ? ' billede valgt' : ' billeder valgt')],
    [/^(\d[\d.,\u00a0]*) without a place$/, (m, n) => n + ' uden sted'],
    [/^(\d[\d.,\u00a0]*) photos? can borrow a place$/, (m, n) => n + (n === '1' ? ' billede' : ' billeder') + ' kan l\u00e5ne et sted'],
    [/^(\d[\d.,\u00a0]*) places borrowed so far$/, (m, n) => n + ' steder l\u00e5nt indtil nu'],
    [/^Set the place of (\d[\d.,\u00a0]*) photos$/, (m, n) => 'S\u00e6t stedet for ' + n + ' billeder'],
    [/^Place set for (\d[\d.,\u00a0]*) photos?( \u00b7 (\d[\d.,\u00a0]*) kept the camera\u2019s)?$/, (m, n, x, k) => 'Sted sat p\u00e5 ' + n + (n === '1' ? ' billede' : ' billeder') + (x ? ' \u00b7 ' + k + ' beholdt kameraets' : '')],
    [/^Place removed from (\d[\d.,\u00a0]*) photos$/, (m, n) => 'Sted fjernet fra ' + n + ' billeder'],
    [/^(\d[\d.,\u00a0]*) photos got a place$/, (m, n) => n + ' billeder fik et sted'],
    [/^(\d[\d.,\u00a0]*) photos · (scanned (.*)|not scanned yet)$/, (m, n, x, ago) => n + ' billeder · ' + (ago ? 'gennemsøgt ' + (tr(ago) || ago) : 'ikke gennemsøgt endnu')],
    [/^Hide (\d[\d.,\u00a0]*) faces as strangers$/, (m, n) => 'Skjul ' + n + ' ansigter som fremmede'],
    [/^The (\d[\d.,\u00a0]*) photos$/, (m, n) => 'De ' + n + ' billeder'],
    [/^See all (\d[\d.,\u00a0]*) photos in this group →$/, (m, n) => 'Se alle ' + n + ' billeder i denne gruppe →'],
    [/^Faces · (\d[\d.,\u00a0]*)$/, (m, n) => 'Ansigter · ' + n],
    [/^Pictures of (.*) · (\d[\d.,\u00a0]*)$/, (m, nm, n) => 'Billeder af ' + nm + ' · ' + n],
    [/^(.*) · (\d[\d.,\u00a0]*) faces$/, (m, nm, n) => (tr(nm) || nm) + ' · ' + n + ' ansigter'],
    [/^Yes, a picture of (.*)$/, (m, nm) => 'Ja, et billede af ' + nm],
    [/^(.*) on the wall\?$/, (m, x) => (tr(x) || x) + ' på væggen?'],
    [/^No one matches “(.*)”$/, (m, q) => 'Ingen matcher “' + q + '”'],
    [/^Last error: (.*)$/, (m, e) => 'Seneste fejl: ' + e],
    [/^moved away (.*)$/, (m, a) => 'flyttet ' + (tr(a) || a)],
    [/^Default \((.*)\)$/, (m, x) => 'Standard (' + x + ')'],
    [/^looks ~(\d+)$/, (m, a) => 'ligner ~' + a],
    [/^· looks ~(\d+)$/, (m, a) => '· ligner ~' + a],
    [/^Marked NSFW \((\d+)% sure\)$/, (m, p) => 'Markeret som NSFW (' + p + ' % sikker)'],
    [/^(.*) · a scanner’s date$/, (m, x) => (tr(x) || x) + ' · en scanners dato'],
    [new RegExp('^(' + Object.keys(MONTHS).join('|') + ') (\\d{4})$'), (m, mo, y) => MONTHS[mo][0].toUpperCase() + MONTHS[mo].slice(1) + ' ' + y],
    [new RegExp('^' + NUM + ' photos?$'), (m, n) => n.trim() + ' ' + n1(n, 'billede', 'billeder')],
    [new RegExp('^' + NUM + ' faces?$'), (m, n) => n.trim() + ' ' + n1(n, 'ansigt', 'ansigter')],
    [new RegExp('^' + NUM + ' groups?$'), (m, n) => n.trim() + ' ' + n1(n, 'gruppe', 'grupper')],
    [new RegExp('^' + NUM + ' photos · ' + NUM + ' groups?$'), (m, a, b) => a.trim() + ' billeder · ' + b.trim() + ' ' + n1(b, 'gruppe', 'grupper')],
    [new RegExp('^' + NUM + ' (faces?) in ' + NUM + ' (groups?) to go$'), (m, a, f, b) => a.trim() + ' ' + n1(a, 'ansigt', 'ansigter') + ' i ' + b.trim() + ' ' + n1(b, 'gruppe', 'grupper') + ' tilbage'],
    [new RegExp('^' + NUM + ' (faces?) to go$'), (m, a) => a.trim() + ' ' + n1(a, 'ansigt', 'ansigter') + ' tilbage'],
    [new RegExp('^GO: file ' + NUM + ' (groups?) \\(' + NUM + ' faces\\)$'), (m, a, g, b) => 'GO: arkivér ' + a.trim() + ' ' + n1(a, 'gruppe', 'grupper') + ' (' + b.trim() + ' ansigter)'],
    [new RegExp('^' + NUM + ' faces in ' + NUM + ' (groups?)$'), (m, a, b) => a.trim() + ' ansigter i ' + b.trim() + ' ' + n1(b, 'gruppe', 'grupper')],
    [new RegExp('^· ' + NUM + ' left for you: as alike to two people$'), (m, a) => '· ' + a.trim() + ' overladt til dig: ligner to personer lige meget'],
    [/^Filed (\d[\d.,\u00a0]*) (groups?) \((\d[\d.,\u00a0]*) faces\): (.*)$/, (m, a, g, b, rest) => 'Arkiverede ' + a.trim() + ' ' + n1(a, 'gruppe', 'grupper') + ' (' + b.trim() + ' ansigter): ' + rest],
    [/^Couldn’t (save|copy|move|rename|remove|undo|look|file them|load Discover|set picture|put it back|turn it): (.*)$/, (m, what, e) => ({save: 'Kunne ikke gemme', copy: 'Kunne ikke kopiere', move: 'Kunne ikke flytte', rename: 'Kunne ikke omdøbe', remove: 'Kunne ikke fjerne', undo: 'Kunne ikke fortryde', look: 'Kunne ikke se', 'file them': 'Kunne ikke arkivere dem', 'load Discover': 'Kunne ikke indlæse Opdag', 'set picture': 'Kunne ikke sætte billede', 'put it back': 'Kunne ikke lægge det tilbage', 'turn it': 'Kunne ikke dreje det'})[what] + ': ' + trServer(e)],
    [/^Couldn’t save the answer(?: for (.*))?: (.*)$/, (m, who, e) => 'Kunne ikke gemme svaret' + (who ? ' for ' + who : '') + ': ' + trServer(e)],
    [/^Moved to (.:\\.*|\/.*)$/, (m, p) => 'Flyttet til ' + p],
    [/^Couldn’t delete: (.*)$/, (m, e) => 'Kunne ikke slette: ' + trServer(e)],
    [/^Saved: (.*)$/, (m, x) => 'Gemt: ' + x.replace(/^(\d+) (faces?) as (.*)$/, (q, n, f, nm) => n + ' ' + n1(n, 'ansigt', 'ansigter') + ' som ' + nm).replace(/^(\d+) faces? of (.*)$/, (q, n, nm) => n + ' ' + n1(n, 'ansigt', 'ansigter') + ' af ' + nm).replace(/^(.*) is (.*)’s (.*)$/, '$1 er $2s $3')],
    [/^Confirmed: (.*)$/, (m, x) => 'Bekræftet: ' + x],
    [/^Confirmed (\d+) faces? of (.*)$/, (m, n, nm) => 'Bekræftede ' + n + ' ' + n1(n, 'ansigt', 'ansigter') + ' af ' + nm],
    [/^None are (.*)$/, (m, nm) => 'Ingen af dem er ' + nm],
    [/^Not (.*) — noted$/, (m, nm) => 'Ikke ' + nm + ' — noteret'],
    [/^Not (.*)$/, (m, nm) => 'Ikke ' + nm],
    [/^Yes, (.*)$/, (m, nm) => 'Ja, ' + nm],
    [/^Is this really (.*)\?$/, (m, nm) => 'Er det virkelig ' + nm + '?'],
    [/^Is this also (.*)\?$/, (m, nm) => 'Er det også ' + nm + '?'],
    [/^Is this really (.*)\? (\d+)% sure$/, (m, nm, p) => 'Er det virkelig ' + nm + '? ' + p + ' % sikker'],
    [/^Moved to (.*)$/, (m, nm) => 'Flyttet til ' + nm],
    [/^Taken out of (.*)$/, (m, nm) => 'Taget ud af ' + nm],
    [/^A picture of (.*)$/, (m, nm) => 'Et billede af ' + nm],
    [/^Marked as a picture of (.*)$/, (m, nm) => 'Markeret som et billede af ' + nm],
    [/^Merged into (.*)$/, (m, nm) => 'Slået sammen med ' + nm],
    [/^Renamed to (.*)$/, (m, nm) => 'Omdøbt til ' + nm],
    [/^Moved (\d+) (groups?) to (.*)$/, (m, n, g, nm) => 'Flyttede ' + n + ' ' + n1(n, 'gruppe', 'grupper') + ' til ' + nm],
    [/^Hidden as a stranger, in all (\d[\d.,\u00a0]*) photos$/, (m, n) => 'Skjult som fremmed på alle ' + n.trim() + ' billeder'],
    [/^Not a person: (\d+) (faces?) taken out$/, (m, n) => 'Ikke en person: ' + n + ' ' + n1(n, 'ansigt', 'ansigter') + ' taget ud'],
    [/^Copied (\d[\d.,\u00a0]*) photos of (.*?)(, (\d[\d.,\u00a0]*) were already there)?(, (\d[\d.,\u00a0]*) failed)?$/, (m, n, nm, a, s, b, f) => 'Kopierede ' + n.trim() + ' billeder af ' + nm + (s ? ', ' + s.trim() + ' lå der i forvejen' : '') + (f ? ', ' + f.trim() + ' mislykkedes' : '')],
    [/^(Copying|Done:) (.*?) · (\d[\d.,\u00a0]*) of (\d[\d.,\u00a0]*)(.*)$/, (m, st, nm, a, b, rest) => (st === 'Copying' ? 'Kopierer ' : 'Færdig: ') + nm + ' · ' + a.trim() + ' af ' + b.trim() + rest.replace(/ · (\d[\d.,\u00a0]*) copied/, ' · $1 kopieret').replace(/ · (\d[\d.,\u00a0]*) already there/, ' · $1 lå der i forvejen').replace(/ · (\d[\d.,\u00a0]*) failed/, ' · $1 mislykkedes')],
    [/^in (.:\\.*|\/.*)$/, (m, p) => 'i ' + p],
    [/^(Copying|Stopped|Done) · (\d[\d., ]*) of (\d[\d., ]*)(.*)$/, (m, st, a, b, rest) => ({Copying: 'Kopierer · ', Stopped: 'Stoppet · ', Done: 'Færdig · '})[st] + a.trim() + ' af ' + b.trim() + rest.replace(/ · (\d[\d., ]*) copied/, ' · $1 kopieret').replace(/ · (\d[\d., ]*) already there/, ' · $1 lå der i forvejen').replace(/ · (\d[\d., ]*) failed/, ' · $1 mislykkedes')],
    [/^Copied (\d[\d., ]*) photos(, (\d[\d., ]*) were already there)?(, (\d[\d., ]*) failed)?$/, (m, n, a, s, b, f) => 'Kopierede ' + n.trim() + ' billeder' + (s ? ', ' + s.trim() + ' lå der i forvejen' : '') + (f ? ', ' + f.trim() + ' mislykkedes' : '')],
    [/^(\d[\d., ]*) photos · ([\d.,]+ [KMGT]?B)( · (\d[\d., ]*) copies left out)?$/, (m, n, size, x, c) => n.trim() + ' billeder · ' + size + (c ? ' · ' + c.trim() + ' kopier springes over' : '')],
    [/^For example: (.*)$/, (m, p) => 'Fx: ' + p],
    [/^Fetch from, e\.g\. (.*)$/, (m, p) => 'Hent fra, fx ' + p],
    [/^Main folder: (.*)$/, (m, p) => 'Hovedmappe: ' + p],
    // A person not in photos before a date
    [/^(.*) isn’t in photos taken before \(day-month-year\)$/, (m, n) => n + ' er ikke på billeder taget før (dag-måned-år)'],
    [/^(.*) isn’t in any photo taken before then$/, (m, n) => n + ' er ikke på nogen billeder taget før den dato'],
    [/^Take (.*) out of (\d[\d., ]*) faces in photos taken before (.*)\?\n\nThe photos stay\. The faces become one group in Review, known not to be (.*), to name as who they are\.$/,
      (m, n, c, d, n2) => 'Fjern ' + n + ' fra ' + c + ' ansigter på billeder taget før ' + d + '?\n\nBillederne bliver. Ansigterne bliver samlet i én gruppe under Gennemse, mærket som ikke ' + n2 + ', så du kan sætte de rigtige navne på.'],
    [/^Took (.*) out of (\d[\d., ]*) faces: they wait in Review$/, (m, n, c) => 'Fjernede ' + n + ' fra ' + c + ' ansigter: de venter under Gennemse'],
    [/^(\d+) (photos?) · (\d+) pages$/, (m, n, p, s) => n + (p === 'photo' ? ' billede' : ' billeder') + ' · ' + s + ' sider'],
    // Ages, now that faces get one
    [/^· born ~(\d+)$/, (m, y) => '· født ~' + y],
    [/^born ~(\d+) ·$/, (m, y) => 'født ~' + y + ' ·'],
    [/^looked ~(\d+) then$/, (m, n) => 'så ud til ~' + n + ' år dengang'],
    [/^· looks ~(\d+)$/, (m, n) => '· ser ud til ~' + n + ' år'],
    [/^looks ~(\d+)$/, (m, n) => 'ser ud til ~' + n + ' år'],
    [/^born ~(\d+) and ~(\d+)$/, (m, a, b) => 'født ~' + a + ' og ~' + b],
    [/^[^,]+ looks \d+(, [^,]+ looks \d+)*(, born ~\d+)?$/, m => m.replace(/ looks (\d+)/g, ' ser ud til $1 år').replace(', born ~', ', født ~')],
    [/^(.+) is entered as (.+)’s ([a-z -]+), but looks born ~(\d+), and (.+) ~(\d+)$/, (m, b, a, r, by, a2, ay) => b + ' er angivet som ' + a + 's ' + (DA[r] || r) + ', men ser ud til at være født ~' + by + ', og ' + a2 + ' ~' + ay],
    [/^Saved the picture as (.*), next to the video$/, (m, n) => 'Billedet er gemt som ' + n + ', ved siden af videoen'],
    [/^(\d[\d., ]*) (album|albums)$/, (m, n, a) => n + ' album'],
    [/^Add (\d[\d., ]*) photos to an album$/, (m, n) => 'Tilføj ' + n + ' billeder til et album'],
    [/^Made the album “(.*)” with (\d[\d., ]*) (photos?)$/, (m, a, n, p) => 'Lavede albummet “' + a + '” med ' + n + (p === 'photo' ? ' billede' : ' billeder')],
    [/^Added (\d[\d., ]*) (photos?) to “(.*)”$/, (m, n, p, a) => 'Tilføjede ' + n + (p === 'photo' ? ' billede' : ' billeder') + ' til “' + a + '”'],
    [/^Took (\d[\d., ]*) photos out of the album$/, (m, n) => 'Tog ' + n + ' billeder ud af albummet'],
    [/^Delete the album “(.*)”\?\n\nThe photos in it are not deleted\.$/, (m, a) => 'Slet albummet “' + a + '”?\n\nBillederne i det slettes ikke.'],
    [/^(.+) · (\d[\d., ]*) (copy|copies)$/, (m, d, n, c) => (tr(d) ?? d) + ' · ' + n + (c === 'copy' ? ' kopi' : ' kopier')],
    [/^Library copied to (.*)$/, (m, p) => 'Biblioteket er kopieret til ' + p],
    [/^Couldn’t copy the library: (.*)$/, (m, e) => 'Kunne ikke kopiere biblioteket: ' + trServer(e)],
    [/^The last copy of the library failed: (.*)$/, (m, e) => 'Den seneste kopi af biblioteket mislykkedes: ' + e],
    [/^Copy the photos you don’t have yet into (.*), sorted by year, month and camera, instead of adding the folder$/, (m, p) => 'Kopiér de billeder, du ikke har endnu, ind i ' + p + ', sorteret efter år, måned og kamera, i stedet for at tilføje mappen'],
    [/^Copying the new photos from (.*) into (.*)$/, (m, a, b) => 'Kopierer de nye billeder fra ' + a + ' ind i ' + b],
    [/^Fetched (\d[\d., ]*) new photos(, (\d[\d., ]*) failed)?$/, (m, n, x, f) => 'Hentede ' + n.trim() + ' nye billeder' + (f ? ', ' + f.trim() + ' mislykkedes' : '')],
    [/^Couldn’t fetch: (.*)$/, (m, e) => 'Kunne ikke hente: ' + trServer(e)],
    [/^The whole path of a new folder, e\.g\. (.*)$/, (m, p) => 'Hele stien til en ny mappe, fx ' + p],
    [/^Show (\d+) more$/, (m, n) => 'Vis ' + n + ' mere'],
    [/^(\d+)% alike$/, (m, p) => p + ' % ens'],
    [/^(\d+)% alike · (\d[\d.,\u00a0]*) of (\d[\d.,\u00a0]*) (photos?)$/, (m, p, a, b) => p + ' % ens · ' + a.trim() + ' af ' + b.trim() + ' ' + n1(b, 'billede', 'billeder')],
    [/^(\d+)% sure(.*)$/, (m, p, rest) => p + ' % sikker' + trRest(rest)],
    [/^(\d+)% likely(.*)$/, (m, p, rest) => p + ' % sandsynligt' + trRest(rest)],
    [/^Similarity (\d+)%(.*)$/, (m, p, rest) => 'Lighed ' + p + ' %' + trRest(rest.replace(' · GO files it as them', ' · GO arkiverer det som dem'))],
    [/^Looks (\d+) here$/, (m, a) => 'Ser ud til at være ' + a + ' her'],
    [/^Filed as (.*) automatically, not confirmed$/, (m, nm) => 'Arkiveret som ' + nm + ' automatisk, ikke bekræftet'],
    [/^Go to (.*)’s page →$/, (m, nm) => 'Gå til ' + nm + 's side →'],
    [/^Go to (.*)’s page$/, (m, nm) => 'Gå til ' + nm + 's side'],
    [/^It’s (.*) — click\. (.*)$/, (m, nm, rest) => 'Det er ' + nm + ' — klik. ' + tr(rest)],
    [/^Not set \(looks like a (woman|man)\)$/, (m, g) => 'Ikke angivet (ligner en ' + (g === 'woman' ? 'kvinde' : 'mand') + ')'],
    [/^is (.*)’s$/, (m, nm) => 'er ' + nm + 's'],
    [/^is (.*)’s (.*?)( \(inferred\))?$/, (m, nm, rel, inf) => 'er ' + nm + 's ' + (DA[rel] || rel) + (inf ? ' (afledt)' : '')],
    [/^Copy (.*)’s photos to$/, (m, nm) => 'Kopiér ' + nm + 's billeder til'],
    [/^about (\d+) min$/, (m, n) => 'omkring ' + n + ' min'],
    [/^about (\d+) h (\d+) min$/, (m, h, n) => 'omkring ' + h + ' t ' + n + ' min'],
    [/^(\d+) min ago$/, (m, n) => 'for ' + n + ' min siden'],
    [/^(\d+) h ago$/, (m, n) => 'for ' + n + ' t siden'],
    [/^(\d+) days ago$/, (m, n) => 'for ' + n + ' dage siden'],
    [/^~(\d+)$/, (m, n) => '~' + n],
    [/^That’s everything · (.*)$/, (m, rest) => 'Det var alt · ' + tr(rest)],
    [/^Learning · \+(\d[\d.,\u00a0]*) faces$/, (m, n) => 'Lærer · +' + n + ' ansigter'],
    [/^(\d[\d.,\u00a0]*) faces identified \((\d[\d.,\u00a0]*) by you\) · (\d[\d.,\u00a0]*) not$/, (m, a, b, c) => a.trim() + ' ansigter genkendt (' + b.trim() + ' af dig) · ' + c.trim() + ' ikke'],
    [/^Filed (\d[\d.,\u00a0]*) faces by themselves · finding the next questions…$/, (m, n) => 'Arkiverede ' + n.trim() + ' ansigter automatisk · finder de næste spørgsmål…'],
    [/^Hid (\d[\d.,\u00a0]*) (faces?) as strangers, in (\d[\d.,\u00a0]*) photos$/, (m, a, f, b) => 'Skjulte ' + a.trim() + ' ' + n1(a, 'ansigt', 'ansigter') + ' som fremmede på ' + b.trim() + ' billeder'],
    [/^No more likely matches for (.*)$/, (m, nm) => 'Ikke flere sandsynlige match for ' + nm],
    [/^No one named “(.*)”( — name them first)?$/, (m, nm, f) => 'Ingen ved navn “' + nm + '”' + (f ? ' — navngiv dem først' : '')],
    [/^Dated (\d[\d.,\u00a0]*) photos?$/, (m, n) => 'Daterede ' + n.trim() + ' ' + n1(n, 'billede', 'billeder')],
    [/^(.*): born (\d+)$/, (m, nm, y) => nm + ': født ' + y],
    [/^Won’t ask when (.*) was born$/, (m, nm) => 'Spørger ikke, hvornår ' + nm + ' er født'],
    [/^With (.*) · (\d+) (days?), (.*)$/, (m, w, n, d, y) => 'Med ' + w + ' · ' + n + ' ' + n1(n, 'dag', 'dage') + ', ' + y],
    [/^Together in (\d[\d.,\u00a0]*) photos on (\d[\d.,\u00a0]*) days, (.*)$/, (m, a, b, y) => 'Sammen på ' + a.trim() + ' billeder på ' + b.trim() + ' dage, ' + y.split(' · ').map(p => tr(p) ?? p).join(' · ')],
    [/^Merge “(.*)” into “(.*)”\? All of (.*)'s groups will be (.*)'s\.$/, (m, a, b, c, d) => 'Slå “' + a + '” sammen med “' + b + '”? Alle ' + c + 's grupper bliver ' + d + 's.'],
    [/^Stop tracking (.*)\?\n\nFiles are not touched, and ratings\/notes are kept if the photos show up again\.$/, (m, f) => 'Stop med at følge ' + f + '?\n\nFilerne røres ikke, og bedømmelser/noter bevares, hvis billederne dukker op igen.'],
    [/^✓ Connected — (.*) with image support$/, (m, x) => '✓ Forbundet — ' + x + ' med billedunderstøttelse'],
    [/^✗ (.*) has no image support — start llama-server with --mmproj$/, (m, x) => '✗ ' + x + ' understøtter ikke billeder — start llama-server med --mmproj'],
  ];

  // Server messages shown to the user.
  const SERVER = [
    [/^Can't reach the embedding server at (\S+)\. (.*)$/, (m, u, e) => 'Kan ikke nå embedding-serveren på ' + u + '. ' + e],
    [/^Face recognition is off: (.*)$/, (m, e) => 'Ansigtsgenkendelse er slået fra: ' + trServer(e)],
    [/^face analysis unavailable: (.*)$/, (m, e) => 'ansigtsanalyse ikke tilgængelig: ' + e],
    [/^choose a folder to copy to$/, () => 'vælg en mappe at kopiere til'],
    [/^give the whole path of the folder, such as (.*)$/, (m, p) => 'angiv hele stien til mappen, fx ' + p],
    [/^can’t create the folder: (.*)$/, (m, e) => 'kan ikke oprette mappen: ' + e],
    [/^already copying (.*)’s photos$/, (m, nm) => 'kopierer allerede ' + nm + 's billeder'],
    [/^already copying the library into a tidy folder$/, () => 'kopierer allerede biblioteket til en ryddet mappe'],
    [/^can’t find the folder (.*)$/, (m, f) => 'kan ikke finde mappen ' + f],
    [/^the folder to fetch from and the tidy folder can’t be inside each other$/, () => 'mappen der hentes fra og den ryddede mappe må ikke ligge inden i hinanden'],
    [/^no data folder to remember fetched files in$/, () => 'ingen datamappe at huske hentede filer i'],
    [/^the library has no photos to copy$/, () => 'biblioteket har ingen billeder at kopiere'],
    [/^give the album a name$/, () => 'giv albummet et navn'],
    [/^give the date as day-month-year$/, () => 'skriv datoen som dag-måned-år'],
    [/^videos can’t be turned here$/, () => 'videoer kan ikke drejes her'],
    [/^give the picture a name$/, () => 'giv billedet et navn'],
    [/^that isn’t a video$/, () => 'det er ikke en video'],
    [/^pictures can only be saved from videos on this computer$/, () => 'billeder kan kun gemmes fra videoer på denne computer'],
    [/^choose a folder outside the library folder (.*): it would read its own copies$/, (m, f) => 'vælg en mappe uden for biblioteksmappen ' + f + ': ellers ville programmet læse sine egne kopier'],
    [/^(.*) has no photos to copy$/, (m, nm) => nm + ' har ingen billeder at kopiere'],
    [/^choose between 50% and 100%$/, () => 'vælg mellem 50 % og 100 %'],
    [/^give a percentage, such as 75$/, () => 'angiv en procent, fx 75'],
    [/^server unreachable$/, () => 'serveren kan ikke nås'],
    [/^face analysis isn’t available \(no face models\)$/, () => 'ansigtsanalyse er ikke tilgængelig (ingen ansigtsmodeller)'],
    [/^can’t read that picture$/, () => 'kan ikke læse det billede'],
    [/^not found$/, () => 'ikke fundet'],
    [/^(.*): photos on a network share can’t be deleted from here$/, (m, f) => f + ': billeder på et netværksdrev kan ikke slettes herfra'],
    [/^no photos chosen$/, () => 'ingen billeder valgt'],
  ];
  function trServer(s) {
    for (const [re, f] of SERVER) { const m = s.match(re); if (m) return f(...m); }
    return s;
  }

  function trRest(rest) { return rest.split(' · ').map(p => (p ? (tr(p) ?? p) : p)).join(' · '); }
  function tr(s) {
    if (lang !== 'da' || !s) return null;
    const key = s.replace(/\s+/g, ' ').trim();
    if (!key || !/[A-Za-z]/.test(key)) return null;
    if (Object.prototype.hasOwnProperty.call(DA, key)) return DA[key];
    for (const [re, f] of PATTERNS) { const m = key.match(re); if (m) { const r = f(...m); if (r != null) return r; } }
    for (const [re, f] of SERVER) { const m = key.match(re); if (m) return f(...m); }
    if (key.includes(' · ')) {
      const parts = key.split(' · '), out = parts.map(p => tr(p));
      if (out.some(x => x != null)) return parts.map((p, i) => out[i] ?? p).join(' · ');
    }
    return null;
  }

  // Applying it to the page.
  const ATTRS = ['title', 'placeholder', 'aria-label', 'alt'];
  const SKIP = new Set(['SCRIPT', 'STYLE', 'TEXTAREA', 'CODE', 'PRE']);
  const done = new WeakMap(); // node -> the text it was given, so its own changes are told apart
  function skip(el) { return !el || SKIP.has(el.nodeName) || (el.closest && el.closest('[data-no-i18n],[contenteditable]')); }
  function doText(node) {
    const v = node.nodeValue;
    if (done.get(node) === v || skip(node.parentElement)) return;
    const t = tr(v);
    if (t != null) {
      const lead = v.match(/^\s*/)[0], trail = v.match(/\s*$/)[0];
      const nv = lead + t + trail;
      done.set(node, nv);
      if (nv !== v) node.nodeValue = nv;
    }
  }
  const attrDone = new WeakMap();
  function doAttrs(el) {
    if (skip(el)) return;
    let seen = attrDone.get(el);
    for (const a of ATTRS) {
      const v = el.getAttribute(a);
      if (v == null || (seen && seen[a] === v)) continue;
      const t = tr(v);
      if (t != null) {
        seen = seen || {}; seen[a] = t; attrDone.set(el, seen);
        if (t !== v) el.setAttribute(a, t);
      }
    }
  }
  function walk(root) {
    if (root.nodeType === 3) return doText(root);
    if (root.nodeType !== 1) return;
    doAttrs(root);
    const w = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
    for (let n = w.nextNode(); n; n = w.nextNode()) n.nodeType === 3 ? doText(n) : doAttrs(n);
  }
  function start() {
    document.documentElement.lang = lang;
    if (lang !== 'da') return;
    walk(document.body);
    new MutationObserver(ms => {
      for (const m of ms) {
        if (m.type === 'characterData') doText(m.target);
        else if (m.type === 'attributes') doAttrs(m.target);
        else m.addedNodes.forEach(walk);
      }
    }).observe(document.body, {subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ATTRS});
    // Native dialogs (merge, stop tracking a folder).
    const conf = window.confirm.bind(window), al = window.alert.bind(window);
    window.confirm = msg => conf(tr(String(msg)) ?? msg);
    window.alert = msg => al(tr(String(msg)) ?? msg);
    document.title = 'ImageOrganizer';
  }

  window.i18n = {
    lang: () => lang,
    tr: s => tr(s) ?? s,
    setLang(l) {
      try { localStorage.setItem(KEY, l); } catch (e) {}
      location.reload();
    },
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
})();
