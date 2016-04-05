class canvas:

	# set the canvas
	import MySQLdb

	canvas = MySQLdb.connect('localhost', 'root')
	brush  = canvas.cursor()

	try: 

		brush.execute("create database lyrics_net")
		canvas.close()

	except: canvas.close()

	def __init__(self):

		# sketch the outline
		canvas, brush = self.prepare()

		brush.execute("""create table if not exists artists (
									  
					name varchar(255) not null, 	  
								  
					primary key (name) 		  
								  
					)""")

		brush.execute("""create table if not exists albums ( 		
										
					title varchar(255) not null, 			
											
					artist_name varchar(255) not null,  		
										
					primary key (title, artist_name), 				
					foreign key (artist_name) references artists (name) 
										
					)""")

		brush.execute("""create table if not exists songs ( 	    	       
									       
					title varchar(255) not null, 	    	       
										       
					album_title varchar(255) not null, 	    	       
									       
					lyrics text, 			    	       
									       
					primary key (album_title, title),
					foreign key (album_title) references albums (title)
					
					)""")

		canvas.close()

	def prepare(self):

		canvas = self.MySQLdb.connect('localhost', 'root', db='lyrics_net')
		brush  = canvas.cursor()

		return canvas, brush

	def draw(self, query, args):

		canvas, brush = self.prepare()

		brush.execute(query, args)
		canvas.commit()

		canvas.close()

	def get_artists(self):

		canvas, brush = self.prepare()

		brush.execute("select name from artists")

		artists = set([item[0] for item in brush.fetchall()])

		canvas.close()

		return artists

	def get_albums(self, artist):

		canvas, brush = self.prepare()

		brush.execute("select title from albums where artist_name=%s", (artist,))

		albums = set([item[0] for item in brush.fetchall()])

		canvas.close()

		return albums

	def get_songs(self, album):

		canvas, brush = self.prepare()

		brush.execute("select title from songs where album_title=%s", (album,))

		songs = set([item[0] for item in brush.fetchall()])

		canvas.close()

		return songs
